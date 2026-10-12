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

// Spark draws a line over its own filled area, scaled to the largest value,
// for a trend in a small space. {{spark .PerDay}}
func Spark(values []int) template.HTML {
	const w, h = 100.0, 32.0
	var b strings.Builder
	b.WriteString(`<svg class="spark" aria-hidden="true" focusable="false" viewBox="0 0 100 32" preserveAspectRatio="none">`)
	if len(values) > 1 {
		top := 1
		for _, v := range values {
			top = max(top, v)
		}
		step := w / float64(len(values)-1)
		var line strings.Builder
		for i, v := range values {
			y := h - 2 - float64(max(v, 0))*(h-4)/float64(top)
			cmd := "L"
			if i == 0 {
				cmd = "M"
			}
			fmt.Fprintf(&line, "%s%.2f %.2f", cmd, float64(i)*step, y)
		}
		fmt.Fprintf(&b, `<path class="ar" d="%sL100 32L0 32Z"/><path class="ln" d="%s" vector-effect="non-scaling-stroke"/>`, line.String(), line.String())
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// Donut draws a ring split into segments, each classed by the matching word
// in tones, e.g. {{donut "ok warn bad" (ints 3 1 0)}}. An empty one is the
// bare track.
func Donut(tones string, values []int) template.HTML {
	names := strings.Fields(tones)
	total, parts := 0, 0
	for _, v := range values {
		if v > 0 {
			total += v
			parts++
		}
	}
	var b strings.Builder
	b.WriteString(`<svg class="donut" aria-hidden="true" focusable="false" viewBox="0 0 120 120"><circle class="trk" cx="60" cy="60" r="50"/>`)
	gap := 0.0
	if parts > 1 {
		gap = 1.5 // a hairline between segments
	}
	at := 0.0
	for i, v := range values {
		if v <= 0 {
			continue
		}
		tone := "acc"
		if i < len(names) {
			tone = names[i]
		}
		seg := float64(v) * 100 / float64(total)
		fmt.Fprintf(&b, `<circle class="seg %s" cx="60" cy="60" r="50" pathLength="100" stroke-dasharray="%.2f 100" stroke-dashoffset="%.2f" transform="rotate(-90 60 60)"/>`,
			template.HTMLEscapeString(tone), max(seg-gap, 0.5), -at)
		at += seg
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

// Column is one bar of a column chart: a label under it and stacked values.
type Column struct {
	Label  string
	Values []int
}

// Columns draws stacked columns with their labels, scaled to the tallest,
// each stack segment classed by the matching word in tones, e.g.
// {{cols "ok bad" .Days}}. Three faint rules mark quarters of the scale.
func Columns(tones string, cols []Column) template.HTML {
	const slot, bar, h, foot = 24, 14, 100, 16
	names := strings.Fields(tones)
	top := 0
	for _, c := range cols {
		sum := 0
		for _, v := range c.Values {
			sum += max(v, 0)
		}
		top = max(top, sum)
	}
	w := max(len(cols), 1) * slot
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="cols" aria-hidden="true" focusable="false" viewBox="0 0 %d %d">`, w, h+foot)
	for _, y := range []int{h / 4, h / 2, h * 3 / 4} {
		fmt.Fprintf(&b, `<line class="rule" x1="0" x2="%d" y1="%d" y2="%d"/>`, w, y, y)
	}
	fmt.Fprintf(&b, `<line class="base" x1="0" x2="%d" y1="%d" y2="%d"/>`, w, h, h)
	for i, c := range cols {
		x := i*slot + (slot-bar)/2
		y := float64(h)
		fmt.Fprintf(&b, `<g class="col">`)
		for j, v := range c.Values {
			if v <= 0 || top == 0 {
				continue
			}
			tone := "acc"
			if j < len(names) {
				tone = names[j]
			}
			bh := max(float64(v)*(h-6)/float64(top), 3)
			y -= bh
			fmt.Fprintf(&b, `<rect class="%s" x="%d" y="%.2f" width="%d" height="%.2f" rx="2"/>`, template.HTMLEscapeString(tone), x, y, bar, bh)
		}
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle">%s</text></g>`, i*slot+slot/2, h+foot-3, template.HTMLEscapeString(c.Label))
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}
