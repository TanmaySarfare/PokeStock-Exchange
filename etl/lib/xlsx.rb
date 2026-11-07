# frozen_string_literal: true

require "cgi"
require "open3"

# Minimal, dependency-free .xlsx reader.
#
# An .xlsx file is a zip archive of XML. We shell out to the system `unzip`
# to pull individual XML parts out and parse just enough of SpreadsheetML to
# recover cell values. Good enough for a flat table of strings and numbers,
# which is exactly what the Pokémon dataset is.
module Xlsx
  class Error < StandardError; end

  Sheet = Struct.new(:name, :rows) do
    # Returns an array of hashes keyed by header (row 1).
    def records
      header, *body = rows
      body.map { |r| header.each_with_index.to_h { |h, i| [h, r[i] || ""] } }
    end
  end

  module_function

  def read(path)
    raise Error, "file not found: #{path}" unless File.exist?(path)

    shared = shared_strings(path)
    workbook = part(path, "xl/workbook.xml")
    rels = part(path, "xl/_rels/workbook.xml.rels")

    rel_targets = rels.scan(/<Relationship\b[^>]*>/).each_with_object({}) do |tag, acc|
      id = tag[/Id="([^"]+)"/, 1]
      target = tag[/Target="([^"]+)"/, 1]
      acc[id] = target.sub(%r{\A/?xl/}, "")
    end

    workbook.scan(/<sheet\b[^>]*>/).map do |tag|
      name = CGI.unescapeHTML(tag[/name="([^"]+)"/, 1])
      rid = tag[/r:id="([^"]+)"/, 1]
      xml = part(path, "xl/#{rel_targets.fetch(rid)}")
      Sheet.new(name, parse_sheet(xml, shared))
    end
  end

  def part(path, name)
    out, err, status = Open3.capture3("unzip", "-p", path, name)
    raise Error, "unzip failed for #{name}: #{err}" unless status.success?

    out.force_encoding("UTF-8")
  end

  def shared_strings(path)
    xml = part(path, "xl/sharedStrings.xml")
    xml.scan(%r{<si>(.*?)</si>}m).map do |(si)|
      CGI.unescapeHTML(si.scan(%r{<t[^>]*>([^<]*)</t>}).flatten.join)
    end
  end

  def parse_sheet(xml, shared)
    rows = []
    xml.scan(%r{<row\b[^>]*>(.*?)</row>}m) do |(row_xml)|
      cells = {}
      row_xml.scan(%r{<c r="([A-Z]+)\d+"([^>]*?)(?:/>|>(.*?)</c>)}m) do |col, attrs, inner|
        next if inner.nil?

        value = inner[%r{<v>(.*?)</v>}m, 1]
        next if value.nil?

        value = shared[value.to_i] if attrs.include?('t="s"')
        cells[column_index(col)] = CGI.unescapeHTML(value)
      end
      next if cells.empty?

      width = cells.keys.max + 1
      rows << Array.new(width) { |i| cells[i] || "" }
    end
    width = rows.map(&:length).max || 0
    rows.each { |r| r.fill("", r.length...width) }
    rows
  end

  def column_index(letters)
    letters.each_char.reduce(0) { |n, ch| n * 26 + (ch.ord - 64) } - 1
  end
end
