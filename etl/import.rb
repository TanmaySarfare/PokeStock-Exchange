#!/usr/bin/env ruby
# frozen_string_literal: true

# PokéStock Exchange — listing desk.
#
# Reads the Pokémon spreadsheet and produces the JSON the Go exchange boots
# from:
#   data/listings.json  — one tradable security per Pokémon
#   data/typechart.json — attacking-type × defending-type multipliers
#
# Usage:
#   ruby etl/import.rb "/path/to/Pokemon Dataset_1.xlsx" [out_dir]

require "json"
require "fileutils"
require_relative "lib/xlsx"
require_relative "lib/listing"

def abort_usage
  warn "usage: ruby etl/import.rb <dataset.xlsx> [out_dir]"
  exit 1
end

path = ARGV[0] or abort_usage
out_dir = ARGV[1] || File.expand_path("../data", __dir__)

started = Time.now
sheets = Xlsx.read(path)
pokemon_sheet = sheets.find { |s| s.rows.first&.include?("Dex No") } or abort("no Pokémon sheet found")
chart_sheet = sheets.find { |s| s.rows.first.to_s.include?("Attacking") } or abort("no type chart sheet found")

# --- listings ---------------------------------------------------------------
symbolizer = Listing::Symbolizer.new
listings = pokemon_sheet.records
                        .reject { |r| r["Name"].to_s.strip.empty? }
                        .map { |r| Listing.build(r, symbolizer.call(r["Name"])) }

dupes = listings.group_by { |l| l[:symbol] }.select { |_, v| v.size > 1 }.keys
abort("duplicate tickers: #{dupes.inspect}") unless dupes.empty?

# --- type chart -------------------------------------------------------------
header, *rows = chart_sheet.rows
defenders = header[1..-1].map(&:strip)
typechart = rows.each_with_object({}) do |row, acc|
  attacker = row[0].strip
  next if attacker.empty?

  acc[attacker] = defenders.each_with_index.to_h { |d, i| [d, Float(row[i + 1])] }
end

# --- write ------------------------------------------------------------------
FileUtils.mkdir_p(out_dir)
File.write(File.join(out_dir, "listings.json"), JSON.pretty_generate(listings))
File.write(File.join(out_dir, "typechart.json"), JSON.pretty_generate(typechart))

sectors = listings.group_by { |l| l[:sector] }.transform_values(&:size).sort_by { |_, n| -n }
puts "Listed #{listings.size} securities across #{sectors.size} sectors in #{(Time.now - started).round(2)}s"
puts "  largest sector: #{sectors.first[0]} (#{sectors.first[1]} listings)"
puts "  priciest IPO:   #{listings.max_by { |l| l[:ipo_price] }.values_at(:symbol, :name, :ipo_price).join(' ')}"
puts "  cheapest IPO:   #{listings.min_by { |l| l[:ipo_price] }.values_at(:symbol, :name, :ipo_price).join(' ')}"
puts "  wrote #{out_dir}/listings.json, #{out_dir}/typechart.json"
