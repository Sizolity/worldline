// Deprecated: this file is retained as a legacy test fixture and for
// backward compatibility with external callers. The v1 main seed path
// goes through internal/app/mod (loading mod/scenarios/<id>/). New code
// should not call BuildDemoWorld or BuildDemoWorldLines.

package seed

import (
	"github.com/sizolity/worldline/rpg/rule"
	"github.com/sizolity/worldline/rpg/story"
	worldmodel "github.com/sizolity/worldline/world/model"
)

// BuildDemoWorld returns a 西游记 demo world with the given ID.
// Pure data construction — no filesystem I/O.
//
// Deprecated: v1 seed.Seed defaults to SeedFromMod, which loads
// mod/scenarios/xiyou-changan/. BuildDemoWorld is preserved only for
// legacy tests and external callers.
func BuildDemoWorld(worldID string) worldmodel.World {
	narrationRule := rule.Rule{
		ID: "rule-narration", Category: "narration", Level: 0,
		Content: "本世界一律以简体中文白话评话风格叙述，文风带古典西游话本之韵：" +
			"诙谐机智、有禅意而不晦涩，多用四字成语与对仗短句；" +
			"NPC 对白依其身份遣词（妖仙傲、和尚雅、八戒俗、沙僧讷）。所有工具调用与状态更新皆在叙述中自然交代。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"叙述", "语言"},
	}
	combatRule := rule.Rule{
		ID: "rule-combat", Category: "combat", Level: 0,
		Content: "凡攻击命中、神通施展、技能检定皆掷一十面（d20）加属性修正，" +
			"过对方 AC 或难度 DC 方为成功。妖仙施法须报神通名号。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"d20", "战斗"},
	}
	karmaRule := rule.Rule{
		ID: "rule-karma", Category: "ethics", Level: 0,
		Content: "因果报应：擅杀无辜或妄取性命者，功德减损，三藏可念紧箍咒；" +
			"行善积德者，气运加增，遇险有救星。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"因果", "道德"},
	}
	monkRule := rule.Rule{
		ID: "rule-jingu", Category: "class", Level: 0,
		Content: "孙悟空有火眼金睛识破伪装，七十二般变化，筋斗云十万八千里；" +
			"但戴紧箍咒在身，三藏念咒则头痛欲裂，需立即停手。",
		Source:  rule.SourceSystem,
		Enabled: true,
		Tags:    []string{"悟空", "紧箍咒"},
	}

	return worldmodel.World{
		ID:   worldmodel.WorldID(worldID),
		Name: "西游记 · 长安启程",
		Canon: worldmodel.Canon{
			Genre: []string{"中国神话", "西游记", "古典奇幻"},
			Tone:  []string{"白话评话", "诙谐机智", "禅意悠远"},
		},
		Description: "贞观十三年秋，玄奘奉旨西天取经，刚收伏齐天大圣孙悟空为大徒弟。" +
			"师徒方出长安东门，一路向西，前途莫测，妖魔丛生。",
		Entities: map[worldmodel.EntityID]worldmodel.Entity{
			"hero-wukong": {
				ID: "hero-wukong", Type: "character", Name: "孙悟空",
				Description: "花果山水帘洞美猴王，齐天大圣，五百年前大闹天宫，" +
					"被佛祖压于五行山下；今为唐三藏所救，戴紧箍咒，拜为大徒弟。" +
					"使一根如意金箍棒，七十二般变化随心所欲，火眼金睛能识千般妖魔。" +
					"性烈如火，眼里揉不得沙子，最恨三藏念那紧箍咒。",
				Tags: []string{"玩家", "妖仙", "大徒弟"},
				State: map[string]worldmodel.Value{
					"hp":         {Kind: worldmodel.ValueKindNumber, Raw: float64(88)},
					"max_hp":     {Kind: worldmodel.ValueKindNumber, Raw: float64(88)},
					"ac":         {Kind: worldmodel.ValueKindNumber, Raw: float64(18)},
					"level":      {Kind: worldmodel.ValueKindNumber, Raw: float64(5)},
					"class":      {Kind: worldmodel.ValueKindString, Raw: "妖仙"},
					"str":        {Kind: worldmodel.ValueKindNumber, Raw: float64(18)},
					"dex":        {Kind: worldmodel.ValueKindNumber, Raw: float64(17)},
					"con":        {Kind: worldmodel.ValueKindNumber, Raw: float64(16)},
					"int":        {Kind: worldmodel.ValueKindNumber, Raw: float64(13)},
					"wis":        {Kind: worldmodel.ValueKindNumber, Raw: float64(10)},
					"cha":        {Kind: worldmodel.ValueKindNumber, Raw: float64(12)},
					"fali":       {Kind: worldmodel.ValueKindNumber, Raw: float64(20)},
					"jingu":      {Kind: worldmodel.ValueKindBoolean, Raw: true},
					"shenbingqi": {Kind: worldmodel.ValueKindString, Raw: "如意金箍棒"},
				},
			},
			"npc-sanzang": {
				ID: "npc-sanzang", Type: "character", Name: "唐三藏",
				Description: "金蝉子转世，奉唐王李世民旨意西行取经，慈悲为怀，" +
					"见妖怪也愿超度，常因不识真伪误怪悟空。" +
					"凡悟空打杀生灵，必念紧箍咒以儆。",
				Tags: []string{"师父", "凡人", "和尚"},
				State: map[string]worldmodel.Value{
					"hp":          {Kind: worldmodel.ValueKindNumber, Raw: float64(20)},
					"disposition": {Kind: worldmodel.ValueKindString, Raw: "慈悲"},
					"trust":       {Kind: worldmodel.ValueKindNumber, Raw: float64(0.6)},
				},
			},
			"npc-bajie": {
				ID: "npc-bajie", Type: "character", Name: "猪八戒",
				Description: "天蓬元帅因酒醉戏嫦娥被贬下界，投错胎成长嘴大耳之猪妖，" +
					"现皈依三藏为二徒弟，使九齿钉耙。贪吃好色，但关键时不掉链子，" +
					"喜欢撺掇师父念咒整治悟空。",
				Tags: []string{"二徒弟", "妖仙", "同伴"},
				State: map[string]worldmodel.Value{
					"hp":      {Kind: worldmodel.ValueKindNumber, Raw: float64(55)},
					"hunger":  {Kind: worldmodel.ValueKindString, Raw: "时刻饿着"},
					"loyalty": {Kind: worldmodel.ValueKindNumber, Raw: float64(0.7)},
				},
			},
			"npc-shaseng": {
				ID: "npc-shaseng", Type: "character", Name: "沙悟净",
				Description: "卷帘大将因失手打碎琉璃盏，贬流沙河为水怪，后皈依三藏为三徒弟。" +
					"挑担最稳，话少心细，使降妖宝杖。",
				Tags: []string{"三徒弟", "妖仙", "同伴"},
				State: map[string]worldmodel.Value{
					"hp":      {Kind: worldmodel.ValueKindNumber, Raw: float64(60)},
					"loyalty": {Kind: worldmodel.ValueKindNumber, Raw: float64(0.9)},
				},
			},
			"npc-yulong": {
				ID: "npc-yulong", Type: "character", Name: "白龙马",
				Description: "西海龙王三太子敖烈，犯天条被斩前观音菩萨点化，" +
					"化为白马为三藏脚力，鲜少现龙身，但关键时可作翻江倒海之事。",
				Tags: []string{"坐骑", "同伴", "龙"},
				State: map[string]worldmodel.Value{
					"hp":   {Kind: worldmodel.ValueKindNumber, Raw: float64(45)},
					"form": {Kind: worldmodel.ValueKindString, Raw: "马身"},
				},
			},
			"loc-changan-gate": {
				ID: "loc-changan-gate", Type: "location", Name: "长安东门",
				Description: "贞观十三年秋，唐王李世民并文武百官于灞桥送行，旌旗蔽日，箫鼓喧天。" +
					"三藏师徒方过东门，回望长安城阙渐远，前方官道直入西山。",
				Tags: []string{"都城", "起点"},
				State: map[string]worldmodel.Value{
					"lit":    {Kind: worldmodel.ValueKindBoolean, Raw: true},
					"danger": {Kind: worldmodel.ValueKindString, Raw: "无"},
				},
			},
			"loc-liangjie-shan": {
				ID: "loc-liangjie-shan", Type: "location", Name: "两界山",
				Description: "出长安西行数十里所至之山，正是悟空被压五百年的五行山旧址，" +
					"山势险峻，常有山贼出没，亦是大唐与西域之分界。",
				Tags: []string{"荒野", "险地"},
				State: map[string]worldmodel.Value{
					"explored": {Kind: worldmodel.ValueKindBoolean, Raw: false},
					"danger":   {Kind: worldmodel.ValueKindString, Raw: "中"},
				},
			},
			"loc-baigu-ling": {
				ID: "loc-baigu-ling", Type: "location", Name: "白虎岭",
				Description: "西行路上一处荒岭，林深草密，白骨累累。" +
					"民间传言有尸魔白骨夫人盘踞，专啖唐僧肉以求长生不老。",
				Tags: []string{"妖境", "前路"},
				State: map[string]worldmodel.Value{
					"explored": {Kind: worldmodel.ValueKindBoolean, Raw: false},
					"danger":   {Kind: worldmodel.ValueKindString, Raw: "未知"},
				},
			},
		},
		Threads: []worldmodel.WorldThread{
			{
				ID: "thread-xixing", Kind: worldmodel.ThreadKindQuest,
				Title:   "西天取经",
				Summary: "三藏师徒奉旨西行往天竺灵山求取大乘真经，途经十万八千里，九九八十一难。",
				Status:  worldmodel.ThreadStatusActive,
				Tension: 0.25,
			},
			{
				ID: "thread-shitu", Kind: worldmodel.ThreadKindMystery,
				Title:   "师徒嫌隙",
				Summary: "悟空性烈，三藏慈悲，紧箍咒一念一痛。前路漫漫，妖魔难辨，师徒之间的信任正悄然消磨。",
				Status:  worldmodel.ThreadStatusOpen,
				Tension: 0.1,
			},
		},
		EventLog: []worldmodel.WorldEvent{
			{
				ID: "evt-qicheng", Type: worldmodel.EventTypeNote,
				Source: worldmodel.EventSourceUser,
				Description: "话说贞观十三年秋九月，玄奘法师奉唐王旨意，自长安东门启程西行求法。" +
					"齐天大圣孙悟空挑担在前，二徒猪八戒、三徒沙悟净紧随其后，白龙马驮三藏跨步缓行。" +
					"长亭外，唐王亲斟御酒，文武含泪相送。马蹄踏霜，长路漫漫——此一去，便是九九八十一难之始。",
			},
		},
		Rules: []worldmodel.Rule{
			rule.ToModelRule(narrationRule),
			rule.ToModelRule(combatRule),
			rule.ToModelRule(karmaRule),
			rule.ToModelRule(monkRule),
		},
		Clock: worldmodel.WorldClock{
			Current:  worldmodel.WorldTime{Kind: worldmodel.WorldTimeScene, Tick: 1},
			Sequence: 1,
		},
	}
}

// BuildDemoWorldLines returns the starter WorldLine set for the demo world.
//
// Deprecated: v1 seed.Seed defaults to SeedFromMod, which compiles
// worldlines from internal/app/mod.CompileScenarioToWorldLines.
// BuildDemoWorldLines is preserved only for legacy tests.
func BuildDemoWorldLines() []story.WorldLine {
	return []story.WorldLine{{
		ID:           "wl_shitu",
		ThreadID:     worldmodel.ThreadID("thread-shitu"),
		Visibility:   story.VisibilityHidden,
		CurrentStage: "初行",
		Drift:        story.Drift{Scene: 0.05, Day: 0.20, Chapter: 0.40},
		Milestones: []story.Milestone{
			{
				ID: "m_xianxi",
				Condition: story.MilestoneCondition{
					Kind: story.CondThreadTensionGTE,
					Args: map[string]any{"thread_id": "thread-shitu", "threshold": 0.30},
				},
				Effects: []worldmodel.Effect{
					{
						Kind: worldmodel.EffectUpdateEntityState, TargetID: "npc-sanzang",
						Payload: map[string]worldmodel.Value{
							"disposition": {Kind: worldmodel.ValueKindString, Raw: "微疑"},
						},
					},
				},
			},
			{
				ID: "m_jueche",
				Condition: story.MilestoneCondition{
					Kind: story.CondThreadTensionGTE,
					Args: map[string]any{"thread_id": "thread-shitu", "threshold": 0.60},
				},
				Effects: []worldmodel.Effect{
					{
						Kind: worldmodel.EffectUpdateThread, TargetID: "thread-shitu",
						Payload: map[string]worldmodel.Value{
							"status": {Kind: worldmodel.ValueKindString, Raw: worldmodel.ThreadStatusActive},
						},
					},
					{
						Kind: worldmodel.EffectUpdateEntityState, TargetID: "npc-sanzang",
						Payload: map[string]worldmodel.Value{
							"disposition": {Kind: worldmodel.ValueKindString, Raw: "心生芥蒂"},
							"trust":       {Kind: worldmodel.ValueKindNumber, Raw: 0.3},
						},
					},
				},
			},
		},
	}}
}
