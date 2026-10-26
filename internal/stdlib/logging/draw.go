package logging

import (
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sekerahmet/TCB/internal/lang/codes"
	"github.com/sekerahmet/TCB/internal/lang/messages"
)

// Çubuğun görünümü (Türkçe sayı yazımı):
//
//	adım 1200/5000 ▕████▌░░░░░░░░░░░▏ %24 · kayıp 2,31 · öğrenme_oranı 3e-4 · 85,2 adım/sn · kalan 14 dk
//	adım 5000/5000 · kayıp 1,84 · 61 dk 3 sn sürdü
//
// Toplamsız sayaçta çubuk ve kalan süre yerine geçen süre gösterilir.

// barWidth çubuğun hücre sayısıdır.
const barWidth = 16

// partialCell bir hücrenin sekizde biri kadar dolu hâlleridir.
var partialCell = []rune(" ▏▎▍▌▋▊▉")

// stepText "adım 1200/5000" ya da "adım 1200". c.mu tutulur.
func (c *Progress) stepText(lang *codes.Language) string {
	if c.total > 0 {
		return messages.TextIn(lang.Code, messages.H7311, c.step, c.total)
	}
	return messages.TextIn(lang.Code, messages.H7310, c.step)
}

// bar dolu oranı (0–1) hücrelerle çizer.
func bar(ratio float64) string {
	ratio = math.Max(0, math.Min(1, ratio))
	eighths := int(ratio * barWidth * 8)
	full, remaining := eighths/8, eighths%8
	var b strings.Builder
	b.WriteString("▕")
	b.WriteString(strings.Repeat("█", full))
	empty := barWidth - full
	if remaining > 0 {
		b.WriteRune(partialCell[remaining])
		empty--
	}
	b.WriteString(strings.Repeat("░", empty))
	b.WriteString("▏")
	return b.String()
}

// metricParts ölçülerin "ad değer" yazımlarıdır. c.mu tutulur.
func (c *Progress) metricParts() []string {
	out := make([]string, 0, len(c.metrics))
	for _, o := range c.metrics {
		out = append(out, o.shown+" "+metricText(o.value))
	}
	return out
}

// barLine çubuğun şimdiki satırıdır. c.mu tutulur.
func (c *Progress) barLine(now time.Time) string {
	head := c.stepText(codes.Display())
	if c.total > 0 {
		ratio := float64(c.step) / float64(c.total)
		head += " " + bar(ratio) + " %" + strconv.Itoa(int(math.Floor(ratio*100)))
	}
	parts := append([]string{head}, c.metricParts()...)
	if c.rate > 0 {
		parts = append(parts, rateText(c.rate))
	}
	switch {
	case c.total > 0 && c.rate > 0:
		parts = append(parts, messages.Text(messages.H7307, remainingText(float64(c.total-c.step)/c.rate)))
	case c.total == 0:
		parts = append(parts, messages.Text(messages.H7308, formatDuration(now.Sub(c.start).Seconds())))
	}
	return strings.Join(parts, " · ")
}

// lastLine çubuk bitince yazılan özet satırdır. c.mu tutulur.
func (c *Progress) lastLine(now time.Time) string {
	parts := append([]string{c.stepText(codes.Display())}, c.metricParts()...)
	parts = append(parts, messages.Text(messages.H7309, formatDuration(now.Sub(c.start).Seconds())))
	return strings.Join(parts, " · ")
}

// rateText "85,2 adım/sn"; saniyede birden yavaşsa "12,3 sn/adım".
func rateText(h float64) string {
	if h < 1 {
		return messages.Text(messages.H7305, formatNumber(1/h))
	}
	return messages.Text(messages.H7306, formatNumber(h))
}

// show çubuğu sıklık sınırıyla gösterir: terminalde canlı satırı yeniden çizer, değilse
// arada bir düz satır yazar. Yorumlayıcı kilidi altında çağrılır.
func (c *Progress) show(now time.Time) {
	c.writeJobStatus(now, false)
	if c.terminal {
		if c.d.active != c || (!c.lastDraw.IsZero() && now.Sub(c.lastDraw) < terminalInterval) {
			c.dirty.Store(true) // ekrandaki satır eski: bir çıktıdan sonra yeniden çizilirken tazelenir
			return
		}
		c.lastDraw = now
		c.dirty.Store(false)
		c.mu.Lock()
		s := c.barLine(now)
		c.mu.Unlock()
		c.m.UpdateLiveLine(func() {
			c.line = s
			c.draw()
		})
		return
	}
	if now.Sub(c.lastPlain) < plainLineInterval {
		return
	}
	c.lastPlain = now
	c.mu.Lock()
	s := c.barLine(now)
	c.mu.Unlock()
	c.m.WriteOutput(c.w, s+"\n")
}

// writeResult bitiş satırını yazar. Terminalde görünen çubuğun satırı aynı yerde özet satırına
// dönüşür ve (varsa) önce açılmış, bitmemiş çubuk yeniden görünür.
func (c *Progress) writeResult(last string) {
	if !c.terminal || c.d.active != c {
		c.m.WriteOutput(c.w, last+"\n")
		return
	}
	d := c.d
	d.active = c.prev
	for d.active != nil && d.active.isDone() {
		d.active = d.active.prev
	}
	c.m.SetLiveLine(nil)
	c.m.UpdateLiveLine(func() {
		c.line = last
		c.draw()
		_, _ = io.WriteString(c.w, "\n")
		d.onScreen = 0
	})
	if e := d.active; e != nil {
		c.m.SetLiveLine(e)
		c.m.UpdateLiveLine(e.Draw) // Draw: iç çubuk görünürken gelen güncellemeler satırı tazeler
	}
}

// isDone çubuk bitti mi?
func (c *Progress) isDone() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stopped
}

// Clear canlı satırı siler (interp.LiveLine); yorumlayıcı çıktı kilidi altında çağırır.
func (c *Progress) Clear() {
	if n := c.d.onScreen; n > 0 {
		_, _ = io.WriteString(c.w, "\r"+strings.Repeat(" ", n)+"\r")
		c.d.onScreen = 0
	}
}

// Draw canlı satırı yeniden çizer (interp.LiveLine); yorumlayıcı çıktı kilidi altında
// çağırır (ör. yazdır'dan sonra). Son güncelleme sıklık sınırı yüzünden çizilmediyse satır
// önce tazelenir.
func (c *Progress) Draw() {
	if c.dirty.Load() {
		c.mu.Lock()
		c.line = c.barLine(clock())
		c.mu.Unlock()
		c.dirty.Store(false)
	}
	c.draw()
}

// draw satırı terminal genişliğine kırparak satır başından yazar; önceki satır daha uzunsa
// artanı boşlukla siler. Çıktı kilidi tutulur.
func (c *Progress) draw() {
	if c.line == "" {
		return
	}
	s := c.m.Mask(c.line)
	if r := []rune(s); c.width > 1 && len(r) > c.width-1 {
		s = string(r[:c.width-2]) + "…"
	}
	n := utf8.RuneCountInString(s)
	pad := ""
	if c.d.onScreen > n {
		pad = strings.Repeat(" ", c.d.onScreen-n)
	}
	_, _ = io.WriteString(c.w, "\r"+s+pad)
	c.d.onScreen = n
}
