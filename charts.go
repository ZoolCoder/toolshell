package toolshell

import (
	"fmt"
	"html/template"
	"strings"
)

// The charts are inline SVG built from attributes only, so the CSP needs no
// 'unsafe-inline' and toolshell.css sets every colour and animation. Each
// one is decorative: the page states the same numbers in text.

// Ring draws a donut showing part of total, from 0 to 100%. {{ring 3 4}}
func Ring(part, total int) template.HTML {
	pct := 0
	if total > 0 {
		pct = min(max(part*100/total, 0), 100)
	}
	val := ""
	// A round cap would draw a dot at 0%, so an empty ring has no value arc.
	if pct > 0 {
		val = fmt.Sprintf(`<circle class="val" cx="60" cy="60" r="52" pathLength="100" stroke-dasharray="%d 100" transform="rotate(-90 60 60)"/>`, pct)
	}
	return template.HTML(`<svg class="ring-svg" aria-hidden="true" focusable="false" viewBox="0 0 120 120"><circle class="trk" cx="60" cy="60" r="52"/>` + val + `</svg>`)
}

// Bars draws one bar per value, scaled to the largest. {{bars .PerDomain}}
func Bars(values []int) template.HTML {
	const w, h, gap = 96, 40, 4
	var b strings.Builder
	b.WriteString(`<svg class="bars" aria-hidden="true" focusable="false" viewBox="0 0 96 40" preserveAspectRatio="xMaxYMax meet">`)
	top := 0
	for _, v := range values {
		top = max(top, v)
	}
	n := max(len(values), 1)
	bw := min((w-gap*(n-1))/n, 18)
	x := w - n*bw - (n-1)*gap
	for _, v := range values {
		bh := 2
		if top > 0 && v > 0 {
			bh = max(v*h/top, 4)
		}
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="2"/>`, x, h-bh, bw, bh)
		x += bw + gap
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// Meter draws a stacked bar of values, each segment classed by the matching
// word in tones, e.g. {{meter "ok warn bad" (ints 3 1 0)}}.
func Meter(tones string, values []int) template.HTML {
	names := strings.Fields(tones)
	total := 0
	for _, v := range values {
		total += max(v, 0)
	}
	var b strings.Builder
	b.WriteString(`<svg class="meter" aria-hidden="true" focusable="false" viewBox="0 0 100 6" preserveAspectRatio="none"><rect class="trk" width="100" height="6"/>`)
	if total > 0 {
		x := 0.0
		for i, v := range values {
			if v <= 0 {
				continue
			}
			tone := "acc"
			if i < len(names) {
				tone = names[i]
			}
			seg := float64(v) * 100 / float64(total)
			fmt.Fprintf(&b, `<rect class="%s" x="%.2f" width="%.2f" height="6"/>`, template.HTMLEscapeString(tone), x, seg)
			x += seg
		}
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// Ints turns template arguments into a slice for bars and meter.
func Ints(v ...int) []int { return v }
