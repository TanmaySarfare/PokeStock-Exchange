package market

import (
	"math"
	"sort"
	"strings"
)

// dossier is static anime-character trivia shown on trainer profile pages.
// Facts follow Serebii.net character bios and Bulbapedia; art is local copies
// of Serebii Pokémon Masters / anime stills (see web/img/trainers/).
type dossier struct {
	Image     string
	Region    string
	Hometown  string
	Debut     string
	Specialty string
	Partner   string
	Bio       string
	Trivia    []string
}

var dossiers = map[string]dossier{
	"bot:ash": {
		Image: "/img/trainers/ash.png", Region: "Kanto", Hometown: "Pallet Town",
		Debut: "EP001 — Pokémon, I Choose You!", Specialty: "All types", Partner: "Pikachu",
		Bio: "Ash Ketchum left Pallet Town at age 10 with a stubborn Pikachu and a dream of becoming a Pokémon Master. After eight regional leagues, the Alola Championship, and the World Coronation Series, he is the closest thing the exchange has to a buy-and-hold legend.",
		Trivia: []string{
			"His starter was supposed to be Bulbasaur, Charmander or Squirtle — he overslept, so Oak handed him Pikachu.",
			"He has caught a Pokémon in every region he has visited, true to the motto 'Gotta catch 'em all'.",
			"He became Alola's first Champion and later defeated Leon to win the World Coronation Series.",
			"Team Rocket has been chasing his Pikachu for more than a thousand episodes.",
		},
	},
	"bot:misty": {
		Image: "/img/trainers/misty.png", Region: "Kanto", Hometown: "Cerulean City",
		Debut: "EP001 — Pokémon, I Choose You!", Specialty: "Water", Partner: "Starmie / Gyarados",
		Bio: "Youngest of the Cerulean Gym's four sisters and a Water-type specialist. She traveled with Ash through Kanto, the Orange Islands and Johto, then returned to run the gym — and, on this exchange, the Cerulean Water Fund.",
		Trivia: []string{
			"She is terrified of bug-types, which is why Bug paper never appears in her book.",
			"Her sisters Daisy, Violet and Lily originally ran the gym as the 'Sensational Sisters'.",
			"Togepi hatched from an egg she found and was her partner until it evolved and stayed in Togepi Paradise.",
			"Serebii lists her as a Water-type Gym Leader who later takes on students of her own.",
		},
	},
	"bot:brock": {
		Image: "/img/trainers/brock.png", Region: "Kanto", Hometown: "Pewter City",
		Debut: "EP005 — Showdown in Pewter City", Specialty: "Rock / Ground", Partner: "Onix / Steelix",
		Bio: "Pewter Gym Leader, amateur chef, and the party's unofficial doctor. Brock walked away from the gym to travel as a Pokémon Breeder, then a Pokémon Doctor, and now runs Pewter Rock & Ground Partners.",
		Trivia: []string{
			"He raised his many younger siblings after their father Flint left to train.",
			"Foros on Serebii note his cooking is famous among traveling trainers — and so is his habit of flirting with every Nurse Joy.",
			"His Onix later evolved into Steelix while in his brother's care.",
			"He studied Pokémon medicine in Sinnoh under Nurse Joy of Twinleaf.",
		},
	},
	"bot:gary": {
		Image: "/img/trainers/gary.png", Region: "Kanto", Hometown: "Pallet Town",
		Debut: "EP001 — Pokémon, I Choose You!", Specialty: "Research / momentum", Partner: "Blastoise / Umbreon",
		Bio: "Professor Oak's grandson and Ash's original rival. Gary burned through gym badges, then hung up the league circuit to become a Pokémon researcher — but he still trades like someone who intends to smell you later.",
		Trivia: []string{
			"He started with Squirtle and traveled with a cheerleading squad Ash found insufferable.",
			"He swept the Kanto gyms ahead of Ash and entered the Indigo League as a favorite — with a cheer squad.",
			"After losing to Ash in the Johto League, he switched from battling to field research.",
			"Serebii's bio lists him as a researcher based out of Pallet Town, still one step ahead of Ash in the lab if not on the field.",
		},
	},
	"bot:jessie": {
		Image: "/img/trainers/jessie.png", Region: "Kanto", Hometown: "unknown (Team Rocket)",
		Debut: "EP002 — Pokémon Emergency!", Specialty: "Chaos / theft", Partner: "Wobbuffet / Arbok",
		Bio: "Team Rocket's loud half. Jessie grew up poor, tried Pokémon nursing, contests, and even a brief gym-leader stint in Johto, then settled on blasting off again for Giovanni. Her book is oversized, random, and frequently underwater.",
		Trivia: []string{
			"She was a classmate of James and Meowth at the Team Rocket Academy.",
			"Dustox and later Gourgeist were contest stars — she has more ribbons than the average Coordinator.",
			"Her hair is naturally pink; the Team Rocket uniform is not a fashion accident.",
			"Every scheme ends with 'we're blasting off again', which is also her stop-loss.",
		},
	},
	"bot:james": {
		Image: "/img/trainers/james.png", Region: "Kanto", Hometown: "a manor he'd rather forget",
		Debut: "EP002 — Pokémon Emergency!", Specialty: "Chaos / collection", Partner: "Carnivine / Weezing",
		Bio: "The polite one. James ran away from an arranged marriage, joined Team Rocket, and has been stealing other people's Pokémon (and occasionally buying them lunch) ever since.",
		Trivia: []string{
			"He comes from a wealthy family and was engaged to Jessiebelle, which is why he fled.",
			"Cacnea, Crawdaunt and Carnivine have all hugged him against his will — a running gag on Serebii's episode guides.",
			"He has a surprisingly deep knowledge of rare bottle caps and vintage porcelain.",
			"Mime Jr. was his contest partner during the Kanto Battle Frontier arc.",
		},
	},
	"bot:meowth": {
		Image: "/img/trainers/meowth.png", Region: "Kanto", Hometown: "Hollywood (failed)",
		Debut: "EP002 — Pokémon Emergency!", Specialty: "Pay Day / talking his way out", Partner: "himself",
		Bio: "The only Meowth who learned to talk — and to walk upright — in a doomed bid to impress a female Meowth named Meowzie. When that failed he joined Team Rocket, where he is translator, mechanic, and the only one who can read a balance sheet.",
		Trivia: []string{
			"He taught himself human speech; the cost was never learning Pay Day as a battle move... until later generations.",
			"Serebii notes he is Team Rocket's tactician when Jessie and James are too busy arguing.",
			"He has a sibling-like rivalry with Ash's Pikachu and a genuine soft spot for other abandoned Pokémon.",
			"Persian, Giovanni's, is the boss he both fears and wants to impress.",
		},
	},
	"bot:oak": {
		Image: "/img/trainers/oak.png", Region: "Kanto", Hometown: "Pallet Town",
		Debut: "EP001 — Pokémon, I Choose You!", Specialty: "Index / research", Partner: "Dragonite (as a young trainer)",
		Bio: "Professor Samuel Oak, inventor of the Pokédex and Pallet Town's starter-hander. He was a league-caliber trainer in his youth (the Special episode 'The Rivalry Reunion' and Origins cover this) and now runs the Pallet Town Index — a little of everything, rebalanced quarterly.",
		Trivia: []string{
			"He and Agatha of the Elite Four were rivals; he chose research over the Champion title.",
			"Every Kanto trainer's first Pokémon still comes from his laboratory.",
			"Serebii's bio lists him as the regional Pokémon Professor and Gary's grandfather.",
			"He is routinely late, snack-obsessed, and better at commentary than at answering his own video phone.",
		},
	},
	"bot:cynthia": {
		Image: "/img/trainers/cynthia.png", Region: "Sinnoh", Hometown: "Celestic Town",
		Debut: "DP040 — Top-Down Training!", Specialty: "Legendary / Mythical paper", Partner: "Garchomp",
		Bio: "Sinnoh Champion, mythologist, and the most dangerous smile on the leaderboard. Cynthia researches the Sinnoh creation myths when she isn't quietly accumulating thin-float Legendary listings.",
		Trivia: []string{
			"Her Garchomp is one of the most decorated ace Pokémon in the anime.",
			"She helped Ash during the Team Galactic crisis and later served as a World Coronation Series finalist.",
			"Serebii lists her research focus as the legends of Arceus, Dialga and Palkia.",
			"She will happily eat a sandwich with you and then 6-0 your entire team.",
		},
	},
	"bot:lance": {
		Image: "/img/trainers/lance.png", Region: "Kanto / Johto", Hometown: "Blackthorn City",
		Debut: "EP063 — Battle of the Badge", Specialty: "Dragon / Flying", Partner: "Dragonite",
		Bio: "Dragon-type Elite Four member (later Champion in some game continuity) who dresses like a caped accountant of destruction. On the exchange he only touches Dragon and Flying paper.",
		Trivia: []string{
			"He is Clair's cousin and a member of the Blackthorn dragon clan.",
			"In the anime he investigated Team Rocket's experiments on Red Gyarados at the Lake of Rage.",
			"Serebii notes his Dragonite can use moves no ordinary Dragonite should know.",
			"He has a soft spot for salvaging Pokémon that Team Rocket has weaponized.",
		},
	},
	"bot:giovanni": {
		Image: "/img/trainers/giovanni.png", Region: "Kanto", Hometown: "Viridian City",
		Debut: "EP063 — Battle of the Badge", Specialty: "Ground / crime", Partner: "Persian / Mewtwo",
		Bio: "Viridian Gym Leader in public, Team Rocket boss in private. Giovanni treats the market the way he treats gym challengers: let them think they're winning, then close the ground from under them.",
		Trivia: []string{
			"He funded the creation of Mewtwo, then lost control of it — twice, depending on the movie you count.",
			"Persian is both his partner and the office cat; Meowth is terrified of it.",
			"Serebii's gym listing still has him as the Ground-type leader of Viridian.",
			"He once froze Team Rocket's operations to focus on 'greater plans', which on this exchange means a concentrated momentum book.",
		},
	},
	"bot:dawn": {
		Image: "/img/trainers/dawn.png", Region: "Sinnoh", Hometown: "Twinleaf Town",
		Debut: "DP001 — Following a Maiden Voyage!", Specialty: "Contests / contrarian", Partner: "Piplup",
		Bio: "Sinnoh Coordinator and Ash's traveling partner through the Sinnoh region. Dawn's Piplup started stubborn and stayed stubborn; her book buys the dip the way she used to buy last-minute contest ribbons.",
		Trivia: []string{
			"Her mother Johanna is a former Super Contest idol — the family business is performance.",
			"Piplup refused to go back in its Poké Ball for most of early Sinnoh, per Serebii's episode notes.",
			"She won the Wallace Cup and several Super Contests before taking on the Grand Festival.",
			"Buneary, Pachirisu and Togekiss were signature performers on her contest team.",
		},
	},
	"bot:may": {
		Image: "/img/trainers/may.png", Region: "Hoenn", Hometown: "Petalburg City",
		Debut: "AG001 — Get the Show on the Road!", Specialty: "Fire / Grass contests", Partner: "Blaziken / Beautifly",
		Bio: "Daughter of Petalburg Gym Leader Norman. May set out wanting a ribbon more than a badge, became one of Hoenn and Kanto's top Coordinators, and now runs a Fire-and-Grass growth fund.",
		Trivia: []string{
			"She originally disliked Pokémon and was going on the journey to 'find herself', not to battle.",
			"Her Torchic evolved all the way to Blaziken, a first for an anime companion starter.",
			"Serebii's contest records have her as a Grand Festival finalist in both Hoenn and Kanto.",
			"She later competed in Johto contests with a newly caught Glaceon.",
		},
	},
	"bot:serena": {
		Image: "/img/trainers/serena.png", Region: "Kalos", Hometown: "Vaniville Town",
		Debut: "XY001 — Kalos, Where Dreams and Adventures Begin!", Specialty: "Fairy / Fire", Partner: "Braixen / Sylveon",
		Bio: "Kalos Queen runner-up and Ash's childhood friend from Professor Oak's summer camp. Serena trades Fairy and Fire the way she performed — with a lot of presentation and a hidden Steel-type will.",
		Trivia: []string{
			"She met Ash at Oak's Pokémon Summer Camp and still has the handkerchief he gave her.",
			"Her Fennekin line (Braixen) was both a performer and a battler, unusual for Kalos Showcases.",
			"Serebii lists her as a Pokémon Performer who later left to compete in Hoenn contests.",
			"She kissed Ash goodbye at the Kalos airport, a first for the anime's main heroine.",
		},
	},
	"bot:steven": {
		Image: "/img/trainers/steven.png", Region: "Hoenn", Hometown: "Mossdeep City",
		Debut: "AG083 — A Shroomish Skirmish (game cameo earlier)", Specialty: "Steel / Rock", Partner: "Metagross",
		Bio: "Hoenn Champion, Devon Corporation heir, and a man who would rather talk about rare stones than about being Champion. His fund is Steel and Rock with museum-quality concentration.",
		Trivia: []string{
			"He collects rare stones the way other Champions collect ribbons.",
			"Metagross is his ace in both the games and the anime's Hoenn championship defenses.",
			"Serebii notes he often wanders caves in disguise rather than sit in the Champion's chamber.",
			"His family runs Devon Corp, which manufactures Poké Navs and, apparently, very patient accountants.",
		},
	},
	"bot:ltsurge": {
		Image: "/img/trainers/surge.jpg", Region: "Kanto", Hometown: "Vermilion City",
		Debut: "EP014 — Electric Shock Showdown", Specialty: "Electric", Partner: "Raichu",
		Bio: "Vermilion Gym Leader, self-described war veteran, and the man who taught Ash that not every Electric-type problem is solved by a Pikachu. He only buys Electric paper.",
		Trivia: []string{
			"His Raichu defeated Ash's Pikachu in their first match by simply outlasting it.",
			"Ash won the rematch after Pikachu refused to evolve, using speed over raw power.",
			"Serebii lists the Trash Badge as his, and the gym as a converted power plant.",
			"He speaks in military metaphors and treats every rally like an amphibious landing.",
		},
	},
	"bot:sabrina": {
		Image: "/img/trainers/sabrina.png", Region: "Kanto", Hometown: "Saffron City",
		Debut: "EP022 — Abra and the Psychic Showdown", Specialty: "Psychic / Ghost", Partner: "Alakazam / Kadabra",
		Bio: "Saffron Gym Leader and a psychic who once sealed her own childhood into a doll. Sabrina trades Psychic and Ghost names as if she already knows next tick's print.",
		Trivia: []string{
			"The Saffron gym in the anime was a maze of teleporting tiles and living dolls.",
			"Ash's Haunter made her laugh, which broke her concentration and cost her the badge — Serebii's episode recap still mentions it.",
			"She is one of the few gym leaders whose type specialty is also a personality description.",
			"In the games she is often the seventh Kanto badge; in the anime Ash faced her much earlier.",
		},
	},
	"bot:nurse": {
		Image: "/img/trainers/nurse.png", Region: "Every region", Hometown: "the nearest Pokémon Center",
		Debut: "EP002 — Pokémon Emergency!", Specialty: "Index / healing", Partner: "Chansey / Blissey / Audino / Comfey",
		Bio: "There is a Nurse Joy in every town and they are all family. This Joy runs the Pokémon Center Pension: tiny positions in everything, almost never a concentrated bet, because someone has to stay solvent to heal the rest of the tape.",
		Trivia: []string{
			"The Joy family resemblance is a running gag across every region Serebii catalogues.",
			"Her partner Pokémon changes by region: Chansey in Kanto, Blissey in Hoenn, Audino in Unova, Wigglytuff in Kalos, Comfey in Alola.",
			"Officer Jenny is her cousin in the same joke — every town, same face, different job.",
			"She is one of the few characters Ash never accidentally electrocutes on first meeting. Usually.",
		},
	},
	"bot:jenny": {
		Image: "/img/trainers/jenny.jpg", Region: "Every region", Hometown: "the nearest police station",
		Debut: "EP005 — Showdown in Pewter City", Specialty: "Compliance / index", Partner: "Growlithe / Herdier / Manectric",
		Bio: "Officer Jenny enforces Pokémon bylaws, traffic, and, on this exchange, position limits. Her book is small, diversified, and aggressively allergic to Team Rocket-shaped chaos.",
		Trivia: []string{
			"Like Nurse Joy, every Officer Jenny is a relative — Serebii's recurring-character index files them as a family.",
			"Growlithe is her classic partner; Unova Jennies often use Herdier.",
			"She has arrested Team Rocket more times than the leaderboard has ticks, and they still escape.",
			"One Jenny in Johto used a Slowpoke, which did not help the chase scenes.",
		},
	},
	"bot:paul": {
		Image: "/img/trainers/paul.jpg", Region: "Sinnoh", Hometown: "Veilstone City",
		Debut: "DP002 — Two Degrees of Separation!", Specialty: "Momentum / strength only", Partner: "Electivire / Torterra",
		Bio: "Ash's Sinnoh rival. Paul releases any Pokémon he considers weak, trains the rest like a drill sergeant, and trades the same way: cut losers fast, size up winners.",
		Trivia: []string{
			"He released Chimchar for being 'pathetic' — Ash caught it, and it evolved into Infernape that later beat him.",
			"His brother Reggie is a retired trainer who now battled as a house-sitter in Veilstone.",
			"Serebii's Sinnoh bios list Electivire, Honchkrow, Drapion and Magmortar among his ace inner circle.",
			"He and Ash's final full battle at the Sinnoh League is widely ranked among the anime's best.",
		},
	},
	"bot:iris": {
		Image: "/img/trainers/iris.png", Region: "Unova", Hometown: "Village of Dragons",
		Debut: "BW001 — In the Shadow of Zekrom!", Specialty: "Dragon", Partner: "Haxorus / Dragonite",
		Bio: "Dragon Master in training (and, in later Unova continuity, Unova Champion). Iris called Ash a kid for two seasons, then quietly became one of the strongest Dragon trainers on the tape.",
		Trivia: []string{
			"She can hear the voices of Dragon-types, a Village of Dragons tradition Serebii notes in her bio.",
			"Her Axew was with her from the Village and evolved much later than fans expected.",
			"She won the Junior Cup in Unova and later returned as Champion in Pokémon Journeys.",
			"Excadrill's refusal to listen to her was a whole arc; it remains a cautionary tale about overtrading a stubborn holding.",
		},
	},
	"bot:cilan": {
		Image: "/img/trainers/cilan.png", Region: "Unova", Hometown: "Striaton City",
		Debut: "BW002 — Enter Iris and Axew!", Specialty: "Grass / Bug", Partner: "Pansage / Crustle",
		Bio: "Grass-type Gym Leader of Striaton, professional Connoisseur, and the man who will tell you whether your Pokémon — or your portfolio — is sweet, bitter or over-steeped.",
		Trivia: []string{
			"He shares the Striaton Gym with brothers Chili (Fire) and Cress (Water); challengers pick a brother by type.",
			"Serebii files him as both a Gym Leader and a Pokémon Connoisseur who evaluates compatibility.",
			"He traveled with Ash and Iris through Unova, often providing the calm explanation after a disaster.",
			"His tasting-note taglines on this exchange are not a bit. He really talks like that.",
		},
	},
	"bot:goh": {
		Image: "/img/trainers/goh.jpg", Region: "Kanto", Hometown: "Vermilion City",
		Debut: "JN001 — Enter Pikachu!", Specialty: "Catching / index", Partner: "Cinderace / Grookey / Inteleon",
		Bio: "Ash's Journeys partner and a researcher at Cerise Laboratory. Goh's goal is to catch every Pokémon — Mew first, if possible — so his book looks like a Pokédex with a brokerage account attached.",
		Trivia: []string{
			"He caught over 80 species during Pokémon Journeys, a record Serebii's catch lists still track.",
			"His first catch was a Scorbunny that evolved into Cinderace, plus Grookey and Sobble from the same lab trio.",
			"He and Chloe attend the same school in Vermilion; he skips it constantly to go into the field.",
			"Project Mew recruited him as a candidate, which is why his fund is named after it.",
		},
	},
	"bot:tracey": {
		Image: "/img/trainers/tracey.jpg", Region: "Orange Islands", Hometown: "Tangelo Island",
		Debut: "EP084 — The Lost Lapras", Specialty: "Sketching / contrarian", Partner: "Scyther / Marill / Venonat",
		Bio: "A Pokémon watcher who sketches more than he battles. Tracey joined Ash and Misty through the Orange Islands, then became Professor Oak's assistant in Pallet Town, which is a long way of saying he buys unloved names and waits.",
		Trivia: []string{
			"He considers himself a Pokémon watcher, not a trainer, and travels to observe rather than to collect badges.",
			"Scyther is old, proud, and still his strongest battler — Serebii's Orange Islands bios highlight the partnership.",
			"Marill and Venonat round out a team built for utility (water, powder, sketches) more than for league runs.",
			"He still sends Oak illustrated field notes instead of just Pokédex data.",
		},
	},
}

func (d dossier) ok() bool { return d.Image != "" }

func (e *Engine) dossierFor(id string) dossier { return dossiers[id] }

func trainerSlug(id string) string { return strings.TrimPrefix(id, "bot:") }

// TrainerView is the profile page for one anime-cast trader (or a human).
type TrainerView struct {
	LeaderRow
	Hometown  string        `json:"hometown,omitempty"`
	Debut     string        `json:"debut,omitempty"`
	Specialty string        `json:"specialty,omitempty"`
	Partner   string        `json:"partner,omitempty"`
	Bio       string        `json:"bio,omitempty"`
	Trivia    []string      `json:"trivia,omitempty"`
	Holdings  []HoldingView `json:"holdings"`
	Cash      float64       `json:"cash"`
	Rank      int           `json:"rank"`
	Players   int           `json:"players"`
}

// Trainer returns a profile by slug ("ash") or raw id ("bot:ash"). Nil if unknown.
func (e *Engine) Trainer(slug string) *TrainerView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	id := slug
	if !strings.HasPrefix(id, "bot:") && !strings.Contains(id, "-") {
		id = "bot:" + strings.ToLower(slug)
	}
	p := e.players[id]
	if p == nil {
		// allow lookup by slug for humans? no — humans use /api/me
		for _, o := range e.players {
			if trainerSlug(o.ID) == strings.ToLower(slug) || strings.EqualFold(o.Name, slug) {
				p = o
				break
			}
		}
	}
	if p == nil {
		return nil
	}
	_, net := e.netWorthLocked(p)
	d := e.dossierFor(p.ID)
	v := &TrainerView{
		LeaderRow: LeaderRow{ID: p.ID[:min(8, len(p.ID))], Slug: trainerSlug(p.ID), Name: p.Name,
			NetWorth: net, PnLPct: round2((net/startingCash - 1) * 100), Bot: p.Bot, Tagline: p.Tagline,
			Role: p.Role, Image: d.Image, Region: d.Region},
		Hometown: d.Hometown, Debut: d.Debut, Specialty: d.Specialty, Partner: d.Partner,
		Bio: d.Bio, Trivia: d.Trivia, Cash: p.Cash, Players: len(e.players),
	}
	best := math.Inf(-1)
	for _, pos := range p.Holdings {
		t := e.tickers[pos.Symbol]
		if t == nil {
			continue
		}
		val := round2(t.Price * float64(pos.Qty))
		cost := pos.AvgCost * float64(pos.Qty)
		h := HoldingView{Position: *pos, Name: t.Name, Image: t.Image, Sector: t.Sector, Price: t.Price, Value: val, PnL: round2(val - cost)}
		if cost > 0 {
			h.PnLPct = round2((val/cost - 1) * 100)
		}
		v.Holdings = append(v.Holdings, h)
		if val > best {
			best = val
			v.Top, v.TopName, v.TopImage = t.Symbol, t.Name, t.Image
		}
	}
	sort.Slice(v.Holdings, func(i, j int) bool { return v.Holdings[i].Value > v.Holdings[j].Value })
	v.Rank = 1
	for _, o := range e.players {
		if o.ID == p.ID {
			continue
		}
		if _, on := e.netWorthLocked(o); on > net {
			v.Rank++
		}
	}
	return v
}
