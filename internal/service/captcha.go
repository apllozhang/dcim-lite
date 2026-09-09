package service

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"sync"
	"time"

	"dcim-lite/internal/apperr"
)

type captchaEntry struct {
	code      string
	expiresAt time.Time
}

// CaptchaService is an in-memory 4-digit captcha (single instance).
type CaptchaService struct {
	mu    sync.Mutex
	items map[string]captchaEntry
	ttl   time.Duration
}

func NewCaptchaService() *CaptchaService {
	return &CaptchaService{items: map[string]captchaEntry{}, ttl: 5 * time.Minute}
}

type CaptchaChallenge struct {
	ID    string `json:"id"`
	Image string `json:"image"` // data:image/svg+xml;base64,...
}

func (s *CaptchaService) Issue() (*CaptchaChallenge, error) {
	code := make([]byte, 4)
	for i := 0; i < 4; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return nil, err
		}
		code[i] = byte('0' + n.Int64())
	}
	id := randomID()
	s.mu.Lock()
	s.gc()
	s.items[id] = captchaEntry{code: string(code), expiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()

	svg := renderCaptchaSVG(string(code))
	b64 := base64.StdEncoding.EncodeToString([]byte(svg))
	return &CaptchaChallenge{
		ID:    id,
		Image: "data:image/svg+xml;base64," + b64,
	}, nil
}

func (s *CaptchaService) Verify(id, code string) error {
	if id == "" || code == "" {
		return apperr.New(400, "CAPTCHA_REQUIRED", "请输入验证码")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gc()
	e, ok := s.items[id]
	if !ok {
		return apperr.New(400, "CAPTCHA_INVALID", "验证码已失效，请刷新")
	}
	// one-time
	delete(s.items, id)
	if time.Now().After(e.expiresAt) {
		return apperr.New(400, "CAPTCHA_INVALID", "验证码已失效，请刷新")
	}
	if e.code != code {
		return apperr.New(400, "CAPTCHA_INVALID", "验证码错误")
	}
	return nil
}

func (s *CaptchaService) gc() {
	now := time.Now()
	for k, v := range s.items {
		if now.After(v.expiresAt) {
			delete(s.items, k)
		}
	}
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// renderCaptchaSVG draws digits with light noise; no external font dependency.
func renderCaptchaSVG(code string) string {
	// simple blocky digits via 7-segment-like paths would be long;
	// use large text + noise lines for bot resistance at this level.
	chars := make([]rune, 0, 4)
	for _, r := range code {
		chars = append(chars, r)
	}
	// jitter x positions
	xs := []int{18, 42, 66, 90}
	ys := []int{34, 36, 33, 35}
	rots := []string{"-8", "6", "-4", "10"}
	var digits string
	for i, ch := range chars {
		digits += fmt.Sprintf(
			`<text x="%d" y="%d" font-size="28" font-family="Arial, sans-serif" fill="#222" transform="rotate(%s %d %d)">%c</text>`,
			xs[i], ys[i], rots[i], xs[i], ys[i], ch,
		)
	}
	// noise lines
	noise := ""
	for i := 0; i < 5; i++ {
		y1, _ := rand.Int(rand.Reader, big.NewInt(40))
		y2, _ := rand.Int(rand.Reader, big.NewInt(40))
		noise += fmt.Sprintf(`<line x1="0" y1="%d" x2="120" y2="%d" stroke="#9aa" stroke-width="1"/>`, y1.Int64(), y2.Int64())
	}
	for i := 0; i < 20; i++ {
		x, _ := rand.Int(rand.Reader, big.NewInt(120))
		y, _ := rand.Int(rand.Reader, big.NewInt(40))
		noise += fmt.Sprintf(`<circle cx="%d" cy="%d" r="1" fill="#bbb"/>`, x.Int64(), y.Int64())
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40"><rect width="120" height="40" fill="#f7f7f5"/>%s%s</svg>`,
		noise, digits,
	)
}
