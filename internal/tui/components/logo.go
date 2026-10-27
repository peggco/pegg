package components

import (
	"math"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/tui/layout"
	"github.com/peggco/pegg/internal/tui/styles"
)

const (
	defaultLogoWidth  = 60
	defaultLogoHeight = 16

	infinitySteps     = 380
	infinityTubeSteps = 30
	infinityRadius    = 0.32
	infinityCameraZ   = 6.8
	infinityRamp      = ".,-~:;=!*#$@"

	logoBaseFPS         = 12.0
	logoMaxFPS          = 60.0
	logoTypingGain      = 1.5
	logoTypingDecay     = 1200 * time.Millisecond
	logoTypingSmoothing = 0.70
	logoMaxTypingCPS    = 20.0
	logoMaxFrameStep    = 250 * time.Millisecond
)

type Logo struct {
	Width  int
	Height int

	lastAnimate time.Time
	lastFrame   time.Time
	lastType    time.Time
	speedAt     time.Time
	typingSpeed float64
	phase       float64
}

func NewLogo(w, h int) *Logo {
	return &Logo{Width: w, Height: h}
}

func DefaultLogo() *Logo {
	return &Logo{Width: defaultLogoWidth, Height: defaultLogoHeight}
}

func (lg *Logo) Typed(now time.Time) {
	lg.decayTyping(now)
	if !lg.lastType.IsZero() {
		if dt := now.Sub(lg.lastType).Seconds(); dt > 0 {
			instant := 1 / dt
			if instant > logoMaxTypingCPS {
				instant = logoMaxTypingCPS
			}
			lg.typingSpeed += (instant - lg.typingSpeed) * logoTypingSmoothing
		}
	}
	lg.lastType = now
}

func (lg *Logo) Animate(now time.Time) bool {
	lg.decayTyping(now)
	if lg.lastAnimate.IsZero() {
		lg.lastAnimate = now
		lg.lastFrame = now
		return false
	}

	fps := lg.fps()
	step := now.Sub(lg.lastAnimate)
	if step > logoMaxFrameStep {
		step = logoMaxFrameStep
	}
	lg.lastAnimate = now
	lg.phase += step.Seconds() * 1000 * (fps / logoBaseFPS)

	interval := time.Duration(float64(time.Second) / fps)
	if interval <= 0 {
		interval = time.Millisecond
	}
	if now.Sub(lg.lastFrame) < interval {
		return false
	}
	lg.lastFrame = now
	return true
}

func (lg *Logo) frameInterval(now time.Time) time.Duration {
	lg.decayTyping(now)
	interval := time.Duration(float64(time.Second) / lg.fps())
	if interval <= 0 {
		return time.Millisecond
	}
	return interval
}

func (lg *Logo) fps() float64 {
	fps := logoBaseFPS + lg.typingSpeed*logoTypingGain
	if fps > logoMaxFPS {
		return logoMaxFPS
	}
	return fps
}

func (lg *Logo) decayTyping(now time.Time) {
	if lg.speedAt.IsZero() || now.Before(lg.speedAt) {
		lg.speedAt = now
		return
	}
	dt := now.Sub(lg.speedAt).Seconds()
	if dt <= 0 {
		return
	}
	lg.typingSpeed *= math.Exp(-dt / logoTypingDecay.Seconds())
	if lg.typingSpeed < 0.01 {
		lg.typingSpeed = 0
	}
	lg.speedAt = now
}

func (lg *Logo) HandleKey(*tcell.EventKey) bool { return false }

func (lg *Logo) Draw(s tcell.Screen, bounds layout.Region, _ bool) {
	th := styles.Current()
	FillRegion(s, bounds, ' ', th.Base().Background(th.Background))
	if bounds.Width < 8 || bounds.Height < 4 {
		return
	}

	width, height := bounds.Width, bounds.Height
	pixels := make([]rune, width*height)
	for i := range pixels {
		pixels[i] = ' '
	}
	depth := make([]float64, width*height)
	for i := range depth {
		depth[i] = math.Inf(-1)
	}

	t := lg.phase
	ax := 0.15 + math.Sin(t*.0004)*0.05
	ay := math.Sin(t*.00035) * 0.12
	az := math.Cos(t*.0003) * 0.02
	cax, sax := math.Cos(ax), math.Sin(ax)
	cay, say := math.Cos(ay), math.Sin(ay)
	caz, saz := math.Cos(az), math.Sin(az)

	scaleX := float64(width) * 0.88 * infinityCameraZ / (2 * 2.67)
	scaleY := float64(height) * 0.84 * infinityCameraZ / (2 * 0.935)
	ramp := []rune(infinityRamp)
	tau := 2 * math.Pi

	for i := 0; i < infinitySteps; i++ {
		t := tau * float64(i) / infinitySteps
		cx := 2.35 * math.Sin(t)
		cy := 1.23 * math.Sin(t) * math.Cos(t)

		tx := 2.35 * math.Cos(t)
		ty := 1.23 * math.Cos(2*t)
		tm := math.Hypot(tx, ty)
		if tm == 0 {
			continue
		}
		tx /= tm
		ty /= tm
		nx := -ty
		ny := tx

		for j := 0; j < infinityTubeSteps; j++ {
			u := tau * float64(j) / infinityTubeSteps
			cu, su := math.Cos(u), math.Sin(u)

			px := cx + infinityRadius*nx*cu
			py := cy + infinityRadius*ny*cu
			pz := infinityRadius * su

			rnx := nx * cu
			rny := ny * cu
			rnz := su

			x, y, z := rotate3(px, py, pz, cax, sax, cay, say, caz, saz)
			vnx, vny, vnz := rotate3(rnx, rny, rnz, cax, sax, cay, say, caz, saz)

			invZ := 1 / (infinityCameraZ - z)
			sx := int(math.Floor(float64(width)/2 + x*scaleX*invZ))
			sy := int(math.Floor(float64(height)/2 - y*scaleY*invZ))
			if sx < 0 || sx >= width || sy < 0 || sy >= height {
				continue
			}

			idx := sy*width + sx
			if invZ <= depth[idx] {
				continue
			}
			depth[idx] = invZ

			diffuse := math.Max(0, vnx*.35+vny*(-.55)+vnz*.76)
			brightness := math.Min(1, .12+diffuse*.58)

			wavePhase := t - lg.phase*.00115
			wave := math.Pow((math.Cos(wavePhase)+1)*.5, 5)
			brightness = math.Min(1, brightness+wave*.38)

			ch := int(brightness * float64(len(ramp)-1))
			if ch < 0 {
				ch = 0
			}
			if ch >= len(ramp) {
				ch = len(ramp) - 1
			}
			pixels[idx] = ramp[ch]
		}
	}

	style := th.Base().Foreground(th.Accent).Bold(true)
	row := make([]rune, width)
	for y := 0; y < height; y++ {
		copy(row, pixels[y*width:(y+1)*width])
		DrawText(s, bounds.Left, bounds.Top+y, string(row), style)
	}
}

func rotate3(x, y, z, cax, sax, cay, say, caz, saz float64) (float64, float64, float64) {
	y, z = y*cax-z*sax, y*sax+z*cax
	x, z = x*cay+z*say, -x*say+z*cay
	x, y = x*caz-y*saz, x*saz+y*caz
	return x, y, z
}
