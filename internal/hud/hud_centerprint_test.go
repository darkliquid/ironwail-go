// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

// Centerprint, finale, and intermission overlay tests split from hud_test.go.

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func TestHUDDrawCenterprintTimeoutFromClientTime(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "2")
	testCV.Set("con_notifyfade", "0")
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          11,
	})
	active := &mockRenderContext{}
	h.Draw(active)
	if len(active.fills) <= 2 {
		t.Fatalf("expected centerprint background fills in addition to status bar, got %d fills", len(active.fills))
	}

	h.SetState(State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          13.1,
	})
	expired := &mockRenderContext{}
	h.Draw(expired)
	if len(expired.fills) != 2 {
		t.Fatalf("expected only status bar fills after centerprint expiry, got %d", len(expired.fills))
	}
}

func TestHUDDrawCenterprintTimeoutUsesScrCenterTime(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "0")
	testCV.Set("con_notifyfade", "0")
	testCV.Set("scr_centertime", "3")
	t.Cleanup(func() {
		testCV.Set("scr_centertime", "2")
	})

	cp := NewCenterprint(nil, testCV)
	active := &mockRenderContext{}
	cp.Draw(active, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.5,
	}, 320, 200)
	if got := charactersToString(active.characters); got != "message" {
		t.Fatalf("centerprint before scr_centertime expiry = %q, want message", got)
	}

	expired := &mockRenderContext{}
	cp.Draw(expired, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          13.1,
	}, 320, 200)
	if got := charactersToString(expired.characters); got != "" {
		t.Fatalf("centerprint after scr_centertime expiry = %q, want empty", got)
	}
}

func TestHUDDrawCenterprintPreservesQuakeHighBitGlyphBytes(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "0")
	testCV.Set("con_notifyfade", "0")

	cp := NewCenterprint(nil, testCV)
	rc := &mockRenderContext{}
	message := string([]byte{0x80, ' ', 'b', 'y', ' ', 0x81})
	cp.Draw(rc, State{
		CenterPrint:   message,
		CenterPrintAt: 1,
		Time:          1.5,
	}, 320, 200)

	got := make([]byte, 0, len(rc.characters))
	for _, ch := range rc.characters {
		got = append(got, byte(ch.num))
	}
	if want := []byte{0x80, ' ', 'b', 'y', ' ', 0x81}; !bytes.Equal(got, want) {
		t.Fatalf("centerprint glyph bytes = %v, want %v", got, want)
	}
}

func TestHUDRegularCenterprintSuppressesWhilePaused(t *testing.T) {
	registerCenterprintTestCvars()
	cp := NewCenterprint(nil, testCV)

	mock := &mockRenderContext{}
	cp.Draw(mock, State{
		Paused:        true,
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          10.5,
	}, 320, 200)
	if got := charactersToString(mock.characters); got != "" {
		t.Fatalf("paused centerprint = %q, want empty", got)
	}

	finale := &mockRenderContext{}
	cp.Draw(finale, State{
		Paused:        true,
		Intermission:  2,
		CenterPrint:   "AB",
		CenterPrintAt: 10,
		Time:          11,
	}, 320, 200)
	if got := charactersToString(finale.characters); got != "AB" {
		t.Fatalf("paused finale centerprint = %q, want AB", got)
	}
}

func TestHUDCenterprintFadeTailExtendsLifetime(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "0")
	testCV.Set("con_notifyfade", "1")
	testCV.Set("con_notifyfadetime", "0.5")

	cp := NewCenterprint(nil, testCV)
	active := &mockRenderContext{}
	cp.Draw(active, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.05,
	}, 320, 200)
	if got := charactersToString(active.characters); got != "message" {
		t.Fatalf("centerprint during fade tail = %q, want %q", got, "message")
	}

	expired := &mockRenderContext{}
	cp.Draw(expired, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.6,
	}, 320, 200)
	if got := charactersToString(expired.characters); got != "" {
		t.Fatalf("centerprint after fade tail = %q, want empty", got)
	}
}

func TestHUDCenterprintFadeTailUsesCharacterAlphaDuringLateFade(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "2")
	testCV.Set("con_notifyfade", "1")
	testCV.Set("con_notifyfadetime", "0.5")

	cp := NewCenterprint(nil, testCV)
	fading := &mockRenderContext{}
	cp.Draw(fading, State{
		CenterPrint:   "message",
		CenterPrintAt: 10,
		Time:          12.25,
	}, 320, 200)

	if got := charactersToString(fading.characters); got != "message" {
		t.Fatalf("late fade text = %q, want full message", got)
	}
	if len(fading.alphaCharacters) != len("message") {
		t.Fatalf("late fade alpha characters = %d, want %d", len(fading.alphaCharacters), len("message"))
	}
	for _, ch := range fading.alphaCharacters {
		if math.Abs(float64(ch.alpha)-0.5) > 0.0001 {
			t.Fatalf("late fade alpha character = %+v, want alpha=0.5", ch)
		}
	}
	if len(fading.alphaFills) != 1 {
		t.Fatalf("late fade alpha background fills = %d, want 1", len(fading.alphaFills))
	}
	if got := fading.alphaFills[0]; got.color != centerPrintPanelColor || math.Abs(float64(got.alpha)-0.35) > 0.0001 {
		t.Fatalf("late fade alpha background fill = %+v, want color=%d alpha=0.35", got, centerPrintPanelColor)
	}
}

func TestHUDIntermissionOverlaySuppressesStatusBar(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  1,
		CompletedTime: 125,
		LevelName:     "Unit Test Map",
		Secrets:       2,
		TotalSecrets:  4,
		Monsters:      5,
		TotalMonsters: 8,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)
	if len(mock.fills) != 0 {
		t.Fatalf("expected no status-bar fill draws during intermission, got %d", len(mock.fills))
	}
	if len(mock.characters) == 0 {
		t.Fatal("expected intermission overlay text draw")
	}
}

func TestHUDIntermissionOverlayUsesGraphicLabelsOnly(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.centerprint.completePic = &image.QPic{Width: 100, Height: 24}
	h.centerprint.interPic = &image.QPic{Width: 64, Height: 24}
	h.SetState(State{
		Intermission:  1,
		CompletedTime: 125,
		LevelName:     "Unit Test Map",
		Secrets:       2,
		TotalSecrets:  4,
		Monsters:      5,
		TotalMonsters: 8,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)

	got := charactersToString(mock.characters)
	if strings.Contains(got, "time") || strings.Contains(got, "secrets") || strings.Contains(got, "monsters") {
		t.Fatalf("intermission text drew duplicate labels: %q", got)
	}
	if !strings.Contains(got, "2:05") || !strings.Contains(got, "2/ 4") || !strings.Contains(got, "5/ 8") {
		t.Fatalf("intermission values missing from draw string output: %q", got)
	}
}

func TestHUDIntermissionOverlayCanBeHiddenOutsideGameplayFocus(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.centerprint.completePic = &image.QPic{Width: 100, Height: 24}
	h.centerprint.interPic = &image.QPic{Width: 64, Height: 24}
	h.SetState(State{
		Intermission:            1,
		HideIntermissionOverlay: true,
		CompletedTime:           125,
		Secrets:                 2,
		TotalSecrets:            4,
		Monsters:                5,
		TotalMonsters:           8,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)

	if len(mock.pics) != 0 || len(mock.characters) != 0 {
		t.Fatalf("hidden intermission overlay drew pics=%d chars=%d", len(mock.pics), len(mock.characters))
	}
}

func TestCenterprintIntermissionUsesMenuSpaceOverlayCoordinates(t *testing.T) {
	complete := &image.QPic{Width: 100, Height: 20}
	inter := &image.QPic{Width: 64, Height: 24}
	cp := &Centerprint{
		cvars:       testCV,
		completePic: complete,
		interPic:    inter,
	}
	mock := &mockRenderContext{}
	cp.Draw(mock, State{Intermission: 1}, 1280, 720)

	if len(mock.pics) != 2 {
		t.Fatalf("screen-space pic draw count = %d, want 2 menu-space-aware pic draws", len(mock.pics))
	}

	want := []struct {
		x, y int
		pic  *image.QPic
	}{
		{x: 110, y: 8, pic: complete},
		{x: 0, y: 56, pic: inter},
	}
	if len(mock.pics) < len(want) {
		t.Fatalf("pic draw count = %d, want at least %d", len(mock.pics), len(want))
	}
	for i, expected := range want {
		got := mock.pics[i]
		if got.x != expected.x || got.y != expected.y || got.pic != expected.pic {
			t.Fatalf("pic draw %d = %+v, want %+v", i, got, expected)
		}
	}
	if len(mock.canvasSwitch) == 0 || mock.canvasSwitch[0] != renderer.CanvasMenu {
		t.Fatalf("canvas switches = %v, want first switch to CanvasMenu", mock.canvasSwitch)
	}
}

func TestHUDFinaleOverlayShowsCenterTextWithoutTimeout(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  2,
		CenterPrint:   "Finale line",
		CenterPrintAt: 1,
		Time:          100,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)
	if len(mock.characters) == 0 {
		t.Fatal("expected finale center text draw")
	}
}

func TestHUDFinaleOverlayRevealsCenterTextOverTime(t *testing.T) {
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	base := State{
		Intermission:  2,
		CenterPrint:   "ABCD",
		CenterPrintAt: 1,
	}

	h.SetState(func() State {
		s := base
		s.Time = 1.1
		return s
	}())
	initial := &mockRenderContext{}
	h.Draw(initial)
	if got := charactersToString(initial.characters); got != "" {
		t.Fatalf("initial finale reveal = %q, want empty", got)
	}

	h.SetState(func() State {
		s := base
		s.Time = 1.26
		return s
	}())
	partial := &mockRenderContext{}
	h.Draw(partial)
	if got := charactersToString(partial.characters); got != "AB" {
		t.Fatalf("partial finale reveal = %q, want AB", got)
	}

	h.SetState(func() State {
		s := base
		s.Time = 1.6
		return s
	}())
	full := &mockRenderContext{}
	h.Draw(full)
	if got := charactersToString(full.characters); got != "ABCD" {
		t.Fatalf("full finale reveal = %q, want ABCD", got)
	}
}

func TestHUDCutsceneOverlayUsesTimedReveal(t *testing.T) {
	registerCenterprintTestCvars()
	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  3,
		CenterPrint:   "A\nB",
		CenterPrintAt: 4,
		Time:          4.26,
	})
	mock := &mockRenderContext{}
	h.Draw(mock)
	if got := charactersToString(mock.characters); got != "AB" {
		t.Fatalf("cutscene reveal = %q, want AB", got)
	}
}

func TestHUDFinaleOverlayUsesScrPrintSpeed(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_printspeed", "4")
	t.Cleanup(func() {
		testCV.Set("scr_printspeed", "8")
	})

	h := NewHUD(nil, testCV)
	h.SetScreenSize(320, 200)
	h.SetState(State{
		Intermission:  2,
		CenterPrint:   "ABCD",
		CenterPrintAt: 1,
		Time:          1.26,
	})

	mock := &mockRenderContext{}
	h.Draw(mock)
	if got := charactersToString(mock.characters); got != "A" {
		t.Fatalf("finale reveal with scr_printspeed=4 = %q, want A", got)
	}
}

func TestCenterprintBackgroundModeThreeUsesFullWidthStrip(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "3")

	cp := NewCenterprint(nil, testCV)
	mock := &mockRenderContext{}
	cp.Draw(mock, State{
		CenterPrint:   "HELLO",
		CenterPrintAt: 1,
		Time:          1.1,
	}, 320, 200)

	if len(mock.fills) != 1 {
		t.Fatalf("fill count = %d, want 1", len(mock.fills))
	}
	if got := mock.fills[0]; got.x != 0 || got.w != 320 {
		t.Fatalf("strip fill = %+v, want full-width strip", got)
	}
	if got := mock.fills[0].color; got != centerPrintPanelColor {
		t.Fatalf("strip fill color = %d, want %d", got, centerPrintPanelColor)
	}
}

func TestCenterprintBackgroundAlphaUsesMenuBGAlpha(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_menubgalpha", "0.25")
	t.Cleanup(func() {
		testCV.Set("scr_menubgalpha", "0.7")
	})

	cp := NewCenterprint(nil, testCV)
	mock := &mockRenderContext{}
	cp.Draw(mock, State{
		CenterPrint:   "HELLO",
		CenterPrintAt: 1,
		Time:          1.5,
	}, 320, 200)

	if len(mock.alphaFills) != 1 {
		t.Fatalf("alpha fill count = %d, want 1", len(mock.alphaFills))
	}
	if got := mock.alphaFills[0]; math.Abs(float64(got.alpha)-0.25) > 0.0001 {
		t.Fatalf("alpha fill = %+v, want alpha=0.25", got)
	}
	if got := mock.alphaFills[0].color; got != centerPrintPanelColor {
		t.Fatalf("alpha fill color = %d, want %d", got, centerPrintPanelColor)
	}
}

func TestCenterprintBackgroundModeOneUsesTextboxArt(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("scr_centerprintbg", "1")
	testCV.Set("scr_menubgalpha", "0.25")

	box := &image.QPic{Width: 8, Height: 8}
	cp := &Centerprint{
		cvars: testCV,
		boxPics: map[string]*image.QPic{
			"gfx/box_tl.lmp":  box,
			"gfx/box_ml.lmp":  box,
			"gfx/box_bl.lmp":  box,
			"gfx/box_tm.lmp":  box,
			"gfx/box_mm.lmp":  box,
			"gfx/box_mm2.lmp": box,
			"gfx/box_bm.lmp":  box,
			"gfx/box_tr.lmp":  box,
			"gfx/box_mr.lmp":  box,
			"gfx/box_br.lmp":  box,
		},
	}
	mock := &mockRenderContext{}
	cp.Draw(mock, State{
		CenterPrint:   "HELLO",
		CenterPrintAt: 1,
		Time:          1.5,
	}, 320, 200)

	if len(mock.fills) != 0 || len(mock.alphaFills) != 0 {
		t.Fatalf("mode 1 fallback fills = %d alphaFills = %d, want 0", len(mock.fills), len(mock.alphaFills))
	}
	if len(mock.alphaPics) != 24 {
		t.Fatalf("mode 1 alpha pic count = %d, want 24", len(mock.alphaPics))
	}
	if got := mock.alphaPics[0]; got.x != 120 || got.y != 58 || got.pic != box || math.Abs(float64(got.alpha)-0.25) > 0.0001 {
		t.Fatalf("mode 1 first alpha pic = %+v, want x=120 y=58 alpha=0.25", got)
	}
}

func TestCenterprintYMatchesCanonicalBranches(t *testing.T) {
	registerCenterprintTestCvars()
	testCV.Set("con_notifyfade", "1")
	testCV.Set("con_notifyfadetime", "0.5")

	if got := centerprintY(200, "one\ntwo"); got != 70 {
		t.Fatalf("short centerprint y = %d, want 70", got)
	}
	if got := centerprintY(200, "1\n2\n3\n4\n5"); got != 48 {
		t.Fatalf("long centerprint y = %d, want 48", got)
	}
	if got := (&Centerprint{cvars: testCV}).centerprintFadeTail(); math.Abs(got-0.5) > 0.0001 {
		t.Fatalf("centerprint fade tail = %.2f, want 0.50", got)
	}
	if got := (&Centerprint{cvars: testCV}).centerprintVisualAlpha(State{CenterPrintAt: 10, Time: 12.25}); math.Abs(got-0.5) > 0.0001 {
		t.Fatalf("centerprint visual alpha = %.2f, want 0.50", got)
	}
}
