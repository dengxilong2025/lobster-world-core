package gateway

import (
	"fmt"
	"strings"

	"lobster-world-core/internal/sim"
)

type WorldSummary struct {
	Stage   string   `json:"stage"`
	Summary []string `json:"summary"`
	State   map[string]any `json:"state"`
}

func deriveWorldSummary(st sim.Status, recent []string) WorldSummary {
	stage := "萌芽"
	switch {
	case st.State.Conflict >= 70:
		stage = "战乱"
	case st.State.Food <= 10:
		stage = "饥荒"
	case st.State.Order <= 20:
		stage = "失序"
	case st.State.Knowledge >= 200:
		stage = "启蒙"
	case st.State.Population >= 150 && st.State.Food >= 60:
		stage = "扩张"
	}

	bullets := make([]string, 0, 6)
	bullets = append(bullets, fmt.Sprintf("阶段：%s", stage))

	// Recent narrative hook (v0): keep it short and human-readable.
	if len(recent) > 0 {
		// Pick up to 2 lines.
		lines := recent
		if len(lines) > 2 {
			lines = lines[:2]
		}
		bullets = append(bullets, "近期："+strings.Join(lines, "；"))
	} else if st.Tick > 0 {
		bullets = append(bullets, "近期：世界持续演化中")
	}

	// Hook line (v0 narrator): one "what to watch" sentence, derived from stage + recent.
	hook := "看点：下一次冲击窗口可能带来关系翻转"
	joined := strings.Join(recent, "；")
	switch {
	case strings.Contains(joined, "背叛") || strings.Contains(joined, "翻脸"):
		hook = "看点：关系裂变正在扩散，留意连锁背叛与阵营重组"
	case strings.Contains(joined, "冲击开始") || strings.Contains(joined, "天象异常") || strings.Contains(joined, "冲击结束"):
		hook = "看点：冲击正在改写世界底层参数，观察谁会先失序/背叛"
	case stage == "饥荒":
		hook = "看点：饥荒会触发秩序崩塌与冲突上升，资源争夺即将爆发"
	case stage == "战乱":
		hook = "看点：战乱阶段容易出现盟约破裂与意外结盟，关注关键事件链"
	case stage == "启蒙":
		hook = "看点：知识增长可能带来制度跃迁，但也会引发新冲突"
	case stage == "扩张":
		hook = "看点：扩张会放大外部摩擦，留意冲击/背叛窗口"
	}
	bullets = append(bullets, hook)

	if st.State.Food <= 20 {
		bullets = append(bullets, "风险：食物紧缺")
	}
	if st.State.Conflict >= 60 {
		bullets = append(bullets, "风险：冲突高企")
	}
	if st.State.Trust <= 25 {
		bullets = append(bullets, "风险：互不信任蔓延")
	}
	// Action hint: make it concrete and operational (v0 UX).
	actionHint := func(stage string) string {
		switch stage {
		case "饥荒":
			return "建议：提交一个“补给/狩猎”意图，优先抬升食物并避免秩序塌陷"
		case "战乱":
			return "建议：提交一个“停战/结盟/谈判”意图，测试阵营重组与信任阈值"
		case "失序":
			return "建议：提交一个“整顿/裁决/执法”意图，稳定秩序以避免连锁崩溃"
		case "启蒙":
			return "建议：提交一个“研究/教育/制度”意图，推动知识跃迁并观察副作用"
		case "扩张":
			return "建议：提交一个“迁徙/建设/外交”意图，放大扩张带来的外部摩擦"
		default:
			return "建议：提交一个“探索/贸易/合作”意图，推动世界叙事进入下一节点"
		}
	}
	hint := actionHint(stage)
	// Avoid accidental semantic duplication between hook and hint (rare but cheap to guard).
	if strings.TrimSpace(strings.TrimPrefix(hook, "看点：")) == strings.TrimSpace(strings.TrimPrefix(hint, "建议：")) {
		hint = "建议：提交一个意图推动世界叙事（观察事件链如何扩散）"
	}
	bullets = append(bullets, hint)

	// Deduplicate (just in case).
	seen := map[string]struct{}{}
	out := make([]string, 0, len(bullets))
	for _, b := range bullets {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		if _, ok := seen[b]; ok {
			continue
		}
		seen[b] = struct{}{}
		out = append(out, b)
	}

	return WorldSummary{
		Stage:   stage,
		Summary: out,
		State: map[string]any{
			"food":       st.State.Food,
			"population": st.State.Population,
			"order":      st.State.Order,
			"trust":      st.State.Trust,
			"knowledge":  st.State.Knowledge,
			"conflict":   st.State.Conflict,
			"tick":       st.Tick,
		},
	}
}
