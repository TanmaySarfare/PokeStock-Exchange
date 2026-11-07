# frozen_string_literal: true

require "minitest/autorun"
require_relative "../lib/listing"

class SymbolizerTest < Minitest::Test
  def setup
    @sym = Listing::Symbolizer.new
  end

  def test_short_uppercase_symbols
    assert_equal "BULB", @sym.call("Bulbasaur")
    assert_equal "PIKA", @sym.call("Pikachu")
  end

  def test_collisions_are_resolved
    a = @sym.call("Charmander")
    b = @sym.call("Charmeleon")
    c = @sym.call("Charizard")
    assert_equal 3, [a, b, c].uniq.size
  end

  def test_strips_punctuation
    assert_match(/\A[A-Z0-9]{3,5}\z/, @sym.call("Mr. Mime"))
    assert_match(/\A[A-Z0-9]{3,5}\z/, @sym.call("Farfetch'd"))
    assert_match(/\A[A-Z0-9]{3,5}\z/, @sym.call("Nidoran♀"))
  end
end

class PricingTest < Minitest::Test
  def test_legendaries_trade_at_premium
    plain = Listing.ipo_price(600, "None", "None", 1)
    legend = Listing.ipo_price(600, "Legendary", "None", 1)
    myth = Listing.ipo_price(600, "Mythical", "None", 1)
    assert plain < legend
    assert legend < myth
  end

  def test_higher_bst_costs_more
    assert Listing.ipo_price(300, "None", "None", 1) < Listing.ipo_price(500, "None", "None", 1)
  end

  def test_fast_frail_is_more_volatile_than_bulky
    jumpy = Listing.volatility_for(150, 120, 400, "None") # fast, frail
    stable = Listing.volatility_for(30, 350, 500, "None") # slow, tanky
    assert jumpy > stable
  end

  def test_volatility_is_clamped
    assert_operator Listing.volatility_for(999, 0, 1, "Mythical"), :<=, 0.02
    assert_operator Listing.volatility_for(0, 999, 999, "None"), :>=, 0.0015
  end
end

class BuildTest < Minitest::Test
  RECORD = {
    "Name" => "Charizard", "Dex No" => "6", "Generation" => "1", "Category" => "Flame Pokémon",
    "Type 1" => "Fire", "Type 2" => "Flying", "Ability 1" => "Blaze", "Ability 2" => "",
    "Hidden Ability" => "Solar Power", "HP" => "78", "Attack" => "84", "Defense" => "78",
    "Sp. Atk" => "109", "Sp. Def" => "85", "Speed" => "100", "BST" => "534",
    "Weak 4x" => "Rock", "Weak 2x" => "Water, Electric", "Resists" => "Fire, Grass (¼×)",
    "Immune" => "Ground", "Evolves From" => "Charmeleon", "Evolution Method" => "Level 36",
    "Evolution Stage" => "3", "Evolution Family" => "Charmander", "Height (m)" => "1.7",
    "Weight (kg)" => "90.5", "% Male" => "87.5", "Legendary Status" => "None",
    "Special Group" => "Starter", "Image URL" => "http://x/6.png"
  }.freeze

  def test_build_produces_a_listing
    l = Listing.build(RECORD, "CHRZ")
    assert_equal "CHRZ", l[:symbol]
    assert_equal "Fire", l[:sector]
    assert_equal "Flying", l[:subsector]
    assert_equal %w[Water Electric], l[:weak2x]
    assert_equal 534, l[:stats][:bst]
    assert_equal 1_000_000, l[:shares]
    assert_includes l[:prospectus], "Spun out of Charmeleon"
    assert_includes l[:prospectus], "Hedged against Ground"
  end
end
