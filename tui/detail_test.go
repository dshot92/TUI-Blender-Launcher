package tui

import (
	"strings"
	"testing"
)

// onlineModel returns a model with the info panel shown and the first build
// (online) carrying a download URL.
func onlineModel(url string) *Model {
	m := sampleModel()
	m.List.Cursor = 0 // first build is the online one
	m.List.Builds[0].DownloadURL = url
	showInfoPanel(m)
	m.UpdateWindowSize(150, 40)
	return m
}

func TestDetailBlockShowsDownloadURLForOnlineBuild(t *testing.T) {
	const url = "https://example.com/blender-4.5.1.tar.xz"
	m := onlineModel(url)

	innerW := m.layout.detailBoxWidth - boxBorderFrameWidth
	block := m.detailBlock(m.List.Builds[0], innerW)

	if block.urlLine < 0 {
		t.Fatalf("expected a URL row for an online build")
	}
	if block.url != url {
		t.Fatalf("block.url = %q, want %q", block.url, url)
	}

	// The URL is rendered and appears verbatim when short enough to fit.
	out := plain(m.View())
	if !strings.Contains(out, "URL") {
		t.Fatalf("expected URL label in the info panel:\n%s", out)
	}
	if !strings.Contains(out, url) {
		t.Fatalf("expected the download URL in the info panel:\n%s", out)
	}
}

func TestDetailBlockOmitsURLForLocalBuild(t *testing.T) {
	m := sampleModel()
	m.List.Cursor = 3 // the local build (no DownloadURL)
	showInfoPanel(m)
	m.UpdateWindowSize(150, 40)

	innerW := m.layout.detailBoxWidth - boxBorderFrameWidth
	block := m.detailBlock(m.List.Builds[3], innerW)

	if block.urlLine != -1 {
		t.Fatalf("local build should have no URL row, got line %d", block.urlLine)
	}
	if strings.Contains(plain(m.View()), "URL") {
		t.Fatalf("local build should not render a URL row:\n%s", plain(m.View()))
	}
}

func TestDetailBlockWrapsLongURLToPaneWidth(t *testing.T) {
	long := "https://download.blender.org/release/Blender4.5/blender-4.5.1-linux-x64.tar.xz?extra=1"
	m := onlineModel(long)

	innerW := m.layout.detailBoxWidth - boxBorderFrameWidth
	block := m.detailBlock(m.List.Builds[0], innerW)

	if block.urlRows < 2 {
		t.Fatalf("expected a long URL to wrap, got %d row(s)", block.urlRows)
	}
	for i, line := range block.lines {
		if w := width(line); w > innerW {
			t.Fatalf("line %d width %d exceeds pane width %d: %q", i, w, innerW, plain(line))
		}
	}
}

func TestHardWrap(t *testing.T) {
	if got := hardWrap("abcdef", 3); strings.Join(got, "") != "abcdef" || len(got) != 2 {
		t.Fatalf("hardWrap(abcdef, 3) = %v", got)
	}
	if got := hardWrap("abc", 10); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("hardWrap(abc, 10) = %v", got)
	}
	if got := hardWrap("", 5); len(got) != 1 || got[0] != "" {
		t.Fatalf("hardWrap(\"\", 5) = %v", got)
	}
	for _, line := range hardWrap(strings.Repeat("x", 10), 4) {
		if width(line) > 4 {
			t.Fatalf("hardWrap produced an over-wide line: %q", line)
		}
	}
}

func TestURLAtHitTestSideBySide(t *testing.T) {
	const url = "https://example.com/blender-4.5.1.tar.xz"
	m := onlineModel(url)
	if !m.layout.detailVisible || m.layout.detailStacked {
		t.Fatalf("expected side-by-side detail (visible=%v stacked=%v)",
			m.layout.detailVisible, m.layout.detailStacked)
	}

	innerW := m.layout.detailBoxWidth - boxBorderFrameWidth
	block := m.detailBlock(m.List.Builds[0], innerW)
	left := m.layout.listBoxWidth + paneGap
	x := left + 1 + 4 // somewhere over the URL text
	y := headerBoxHeight + 1 + block.urlLine

	got, ok := m.urlAt(x, y)
	if !ok || got != url {
		t.Fatalf("urlAt(%d, %d) = (%q, %v), want (%q, true)", x, y, got, ok, url)
	}

	// Just above the row and one column to the left of the pane are misses.
	if _, ok := m.urlAt(x, y-1); ok {
		t.Fatalf("click above the URL row should miss")
	}
	if _, ok := m.urlAt(left, y); ok {
		t.Fatalf("click on the border should miss")
	}
	if _, ok := m.urlAt(x+innerW+5, y); ok {
		t.Fatalf("click past the pane should miss")
	}
}

func TestURLAtHitTestStacked(t *testing.T) {
	m := sampleModel()
	m.List.Cursor = 0
	m.List.Builds[0].DownloadURL = "https://example.com/blender-4.5.1.tar.xz"
	showInfoPanel(m)
	m.UpdateWindowSize(90, 40)
	if !m.layout.detailStacked {
		t.Fatalf("expected stacked detail at 90 cols (stacked=%v)", m.layout.detailStacked)
	}

	innerW := m.layout.detailBoxWidth - boxBorderFrameWidth
	block := m.detailBlock(m.List.Builds[0], innerW)
	x := 1 + 2
	y := headerBoxHeight + m.layout.listBoxHeight + 1 + block.urlLine

	if _, ok := m.urlAt(x, y); !ok {
		t.Fatalf("urlAt(%d, %d) should hit the stacked URL row", x, y)
	}
}

func TestHandleOpenURLOnlyWithURL(t *testing.T) {
	m := sampleModel()
	m.List.Cursor = 3 // local build, no URL
	if _, cmd := m.handleOpenURL(); cmd != nil {
		t.Fatalf("local build should not produce an open-URL command")
	}

	m.List.Builds[3].DownloadURL = "https://example.com/x"
	if _, cmd := m.handleOpenURL(); cmd == nil {
		t.Fatalf("build with a URL should produce an open-URL command")
	}
}
