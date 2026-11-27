package cli

import (
	"fmt"
	"os"
	"strings"
)

// The minectl logo, pixel for pixel from web/public/mark.png (a 13×13
// pickaxe) and wordmark-navy.png (55×12). Each character cell holds two
// pixels stacked with half blocks, so the pixels stay square.

var markPixels = []string{
	"....aaaaa....",
	"...abbbbbacc.",
	"....aaaabbde.",
	"........cbba.",
	".......cdebba",
	"......cde.aba",
	".....cde..aba",
	"....cde...aba",
	"...cde....aba",
	"..cde......a.",
	".cde.........",
	"cde..........",
	"de...........",
}

var markColors = map[byte][3]int{
	'a': {70, 135, 255},  // blue
	'b': {113, 240, 244}, // diamond
	'c': {124, 78, 39},   // handle
	'd': {179, 127, 82},  // handle, light
	'e': {61, 36, 13},    // handle, dark
}

// wordPixels starts with a blank row so its two-pixel strokes line up with
// whole character cells.
var wordPixels = []string{
	".......................................................",
	"...............##....................................##",
	"...............##...............................##...##",
	"................................................##...##",
	"######..####...##.#######.....######...#####...####..##",
	"######..####...##.#######.....######...#####...####..##",
	"##....##....##.##.##.....##.##....##.##.....##..##...##",
	"##....##....##.##.##.....##.########.##.....##..##...##",
	"##....##....##.##.##.....##.######...##.........##...##",
	"##....##....##.##.##.....##.##.......##.....##..##...##",
	"##....##....##.##.##.....##.##....##.##.....##..##...##",
	"##....##....##.##.##.....##...######...#####......##.##",
	"##....##....##.##.##.....##...######...#####......##.##",
}

// colorSGR is the escape sequence parameter for an RGB colour: exact on
// terminals that say they support it, else the nearest of the 256 colours.
func colorSGR(c [3]int, background bool) string {
	base := "38"
	if background {
		base = "48"
	}
	if ct := os.Getenv("COLORTERM"); ct == "truecolor" || ct == "24bit" {
		return fmt.Sprintf("%s;2;%d;%d;%d", base, c[0], c[1], c[2])
	}
	q := func(v int) int { return (v*5 + 127) / 255 }
	return fmt.Sprintf("%s;5;%d", base, 16+36*q(c[0])+6*q(c[1])+q(c[2]))
}

// halfBlocks renders pixel rows two at a time. color returns a pixel's
// colour, or ok=false to draw it in the terminal's own text colour.
func halfBlocks(rows []string, colored bool, color func(byte) ([3]int, bool)) []string {
	if len(rows)%2 == 1 {
		rows = append(rows, strings.Repeat(".", len(rows[0])))
	}
	fg := func(p byte) string {
		if c, ok := color(p); ok {
			return "\033[" + colorSGR(c, false) + "m"
		}
		return ""
	}
	var out []string
	for r := 0; r < len(rows); r += 2 {
		var b strings.Builder
		for i := 0; i < len(rows[r]); i++ {
			top, bot := rows[r][i], rows[r+1][i]
			hasTop, hasBot := top != '.', bot != '.'
			switch {
			case !hasTop && !hasBot:
				b.WriteByte(' ')
			case !colored:
				b.WriteString(map[[2]bool]string{{true, true}: "█", {true, false}: "▀", {false, true}: "▄"}[[2]bool{hasTop, hasBot}])
			case hasTop && hasBot && top == bot:
				b.WriteString(fg(top) + "█\033[0m")
			case hasTop && hasBot:
				// Two colours in one cell: the top as text, the bottom as background.
				bg := ""
				if c, ok := color(bot); ok {
					bg = "\033[" + colorSGR(c, true) + "m"
				}
				b.WriteString(fg(top) + bg + "▀\033[0m")
			case hasTop:
				b.WriteString(fg(top) + "▀\033[0m")
			default:
				b.WriteString(fg(bot) + "▄\033[0m")
			}
		}
		out = append(out, b.String())
	}
	return out
}

// fullLogo is the pickaxe and the wordmark side by side, seven lines tall.
// Without colour it keeps the shape in the terminal's text colour.
func fullLogo() []string {
	colored := colorFor(os.Stdout)
	mark := halfBlocks(markPixels, colored, func(p byte) ([3]int, bool) { c, ok := markColors[p]; return c, ok })
	word := halfBlocks(wordPixels, colored, func(byte) ([3]int, bool) { return [3]int{}, false })
	lines := make([]string, len(mark))
	for i := range mark {
		lines[i] = mark[i] + "   " + word[i]
	}
	return lines
}
