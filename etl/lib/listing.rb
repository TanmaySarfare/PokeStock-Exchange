# frozen_string_literal: true

# The "listing desk": turns a raw Pokémon record into a publicly traded
# security. Every number here is derived from the spreadsheet so the market
# has a (tenuous) grounding in the actual data.
module Listing
  TYPES = %w[Normal Fire Water Electric Grass Ice Fighting Poison Ground Flying
             Psychic Bug Rock Ghost Dragon Dark Steel Fairy].freeze

  STATS = ["HP", "Attack", "Defense", "Sp. Atk", "Sp. Def", "Speed"].freeze

  # Price premium for rarity. Legendary paper trades at a premium.
  RARITY_MULT = {
    "Legendary" => 1.6,
    "Mythical" => 2.1,
    "None" => 1.0
  }.freeze

  GROUP_MULT = {
    "Starter" => 1.15,
    "Fossil" => 1.1,
    "Paradox" => 1.35,
    "Ultra Beast" => 1.4,
    "Baby" => 0.9,
    "None" => 1.0
  }.freeze

  # Shares outstanding: rarer listings have a tighter float.
  FLOAT = {
    "Legendary" => 120_000,
    "Mythical" => 40_000,
    "None" => 1_000_000
  }.freeze

  class Symbolizer
    def initialize
      @taken = {}
    end

    # Generates a short, unique, uppercase ticker from a name.
    #   Bulbasaur -> BULB, Charizard -> CHAR, Charmander -> CHRM ...
    def call(name)
      base = name.upcase.gsub(/[^A-Z]/, "")
      base = "PKMN" if base.empty?
      consonants = base[0] + base[1..-1].to_s.gsub(/[AEIOU]/, "")

      candidates = [
        base[0, 4],
        consonants[0, 4],
        base[0, 3] + base[-1],
        base[0, 5],
        consonants[0, 5],
        base[0, 2] + base[-2, 2].to_s
      ].select { |c| c.length >= 4 }
      candidates << consonants[0, 4] # allow a 3-letter consonant symbol as a fallback
      (2..99).each { |i| candidates << base[0, 3] + i.to_s }

      sym = candidates.find { |c| c && c.length >= 3 && !@taken.key?(c) }
      @taken[sym] = name
      sym
    end
  end

  module_function

  def num(value)
    Float(value)
  rescue ArgumentError, TypeError
    nil
  end

  def int(value)
    num(value)&.round
  end

  def list(value)
    value.to_s.split(",").map(&:strip).reject(&:empty?)
  end

  def build(record, symbol)
    stats = STATS.each_with_object({}) { |s, h| h[s] = int(record[s]) || 0 }
    bst = int(record["BST"]) || stats.values.sum
    rarity = record["Legendary Status"].to_s.strip
    rarity = "None" if rarity.empty?
    group = record["Special Group"].to_s.strip
    group = "None" if group.empty?
    stage = int(record["Evolution Stage"]) || 1
    speed = stats["Speed"]
    bulk = stats["HP"] + stats["Defense"] + stats["Sp. Def"]

    price = ipo_price(bst, rarity, group, stage)
    volatility = volatility_for(speed, bulk, bst, rarity)
    drift = drift_for(stage, group)
    shares = FLOAT.fetch(rarity, FLOAT["None"])

    {
      symbol: symbol,
      name: record["Name"],
      dex: int(record["Dex No"]),
      generation: int(record["Generation"]),
      category: record["Category"].to_s,
      sector: record["Type 1"].to_s,
      subsector: record["Type 2"].to_s.empty? ? nil : record["Type 2"].to_s,
      abilities: [record["Ability 1"], record["Ability 2"]].map(&:to_s).reject(&:empty?),
      hidden_ability: record["Hidden Ability"].to_s.empty? ? nil : record["Hidden Ability"].to_s,
      stats: {
        hp: stats["HP"], atk: stats["Attack"], def: stats["Defense"],
        spa: stats["Sp. Atk"], spd: stats["Sp. Def"], spe: speed, bst: bst
      },
      weak4x: list(record["Weak 4x"]),
      weak2x: list(record["Weak 2x"]),
      resists: list(record["Resists"]),
      immune: list(record["Immune"]),
      evolves_from: record["Evolves From"].to_s.empty? ? nil : record["Evolves From"].to_s,
      evolution_method: record["Evolution Method"].to_s.empty? ? nil : record["Evolution Method"].to_s,
      stage: stage,
      family: record["Evolution Family"].to_s,
      height_m: num(record["Height (m)"]),
      weight_kg: num(record["Weight (kg)"]),
      pct_male: num(record["% Male"]),
      rarity: rarity,
      group: group,
      image: record["Image URL"].to_s,
      ipo_price: price,
      volatility: volatility,
      drift: drift,
      shares: shares,
      prospectus: prospectus(record, stats, bst, rarity, group, stage)
    }
  end

  # Price scales super-linearly with BST so top-tier mons feel expensive.
  def ipo_price(bst, rarity, group, stage)
    base = (bst**1.5) / 100.0
    mult = RARITY_MULT.fetch(rarity, 1.0) * GROUP_MULT.fetch(group, 1.0) * (1 + 0.05 * (stage - 1))
    (base * mult).round(2)
  end

  # Per-tick standard deviation of log returns. Fast, frail Pokémon are
  # jumpy; bulky ones are blue chips. Legendaries are thinly traded, so they
  # swing harder.
  def volatility_for(speed, bulk, bst, rarity)
    speed_ratio = bst.zero? ? 0.2 : speed.to_f / bst
    bulk_ratio = bst.zero? ? 0.4 : bulk.to_f / bst
    v = 0.003 + 0.030 * speed_ratio - 0.008 * bulk_ratio
    v *= 1.35 if rarity != "None"
    v.clamp(0.0015, 0.02).round(5)
  end

  # Tiny per-tick drift. Early evolution stages are "growth stocks" with
  # slight upward bias; fully evolved are "value" with flat drift.
  def drift_for(stage, group)
    d = { 1 => 0.00012, 2 => 0.00006, 3 => 0.0 }.fetch(stage, 0.0)
    d += 0.0001 if group == "Baby"
    d.round(6)
  end

  def prospectus(record, stats, bst, rarity, group, stage)
    name = record["Name"]
    type1 = record["Type 1"]
    type2 = record["Type 2"].to_s
    category = record["Category"].to_s.sub(/ Pokémon\z/, "")
    best = stats.max_by { |_, v| v }
    worst = stats.min_by { |_, v| v }
    weight = num(record["Weight (kg)"])
    height = num(record["Height (m)"])

    sentences = []
    sentences << "#{name} is a Generation #{record['Generation']} #{category} concern operating primarily in the #{type1} sector" \
                 "#{type2.empty? ? '' : " with significant #{type2} exposure"}."
    sentences << "Management credits its #{record['Ability 1']} operating model" \
                 "#{record['Hidden Ability'].to_s.empty? ? '' : ", with #{record['Hidden Ability']} held in reserve as a hidden asset"}."
    sentences << "Core competency: #{best[0]} (#{best[1]}). Known weakness on the balance sheet: #{worst[0]} (#{worst[1]}). Total book value (BST): #{bst}."
    if record["Evolves From"].to_s.empty?
      sentences << (stage == 1 && int(record["Evolution Stage"]) ? "Listed as an original issue with no parent company." : "")
    else
      sentences << "Spun out of #{record['Evolves From']} via #{record['Evolution Method']}."
    end
    unless record["Weak 2x"].to_s.empty? && record["Weak 4x"].to_s.empty?
      risks = [record["Weak 4x"].to_s, record["Weak 2x"].to_s].reject(&:empty?).join(", ")
      sentences << "Risk factors: materially exposed to #{risks} market conditions."
    end
    sentences << "Hedged against #{record['Immune']} downturns." unless record["Immune"].to_s.empty?
    sentences << "Physical plant: #{height} m tall, #{weight} kg." if height && weight
    sentences << "Classified as #{rarity} paper; expect a thin float and wide spreads." if rarity != "None"
    sentences << "Part of the #{group} index." if group != "None"
    sentences.reject(&:empty?).join(" ")
  end
end
