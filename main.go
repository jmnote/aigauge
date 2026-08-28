package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend
var embeddedFrontend embed.FS

//go:embed frontend/icon.png
var appIcon []byte

//go:embed VERSION
var versionFile string

var frontendAssets fs.FS

const (
	// createNoWindow prevents console window from appearing when spawning child processes on Windows
	createNoWindow = 0x08000000
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

var singleInstanceKey = [32]byte{
	0x61, 0x69, 0x67, 0x61, 0x75, 0x67, 0x65, 0x2d,
	0x73, 0x69, 0x6e, 0x67, 0x6c, 0x65, 0x2d, 0x69,
	0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2d,
	0x6b, 0x65, 0x79, 0x2d, 0x76, 0x31, 0x2d, 0x30,
}

type CodexUsage struct {
	Plan       string  `json:"plan"`
	FiveHour   float64 `json:"fiveHour"`
	SevenDay   float64 `json:"sevenDay"`
	FiveHourIn int     `json:"fiveHourResetIn"`
	SevenDayIn int     `json:"sevenDayResetIn"`
	FetchedAt  string  `json:"fetchedAt"`
	Error      string  `json:"error,omitempty"`
}

type AntigravityUsage struct {
	Groups    []AntigravityUsageGroup `json:"groups"`
	FetchedAt string                  `json:"fetchedAt"`
	Error     string                  `json:"error,omitempty"`
}

type AntigravityUsageGroup struct {
	Name    string                   `json:"name"`
	Buckets []AntigravityUsageBucket `json:"buckets"`
}

type AntigravityUsageBucket struct {
	Name      string  `json:"name"`
	Window    string  `json:"window"`
	Remaining float64 `json:"remaining"`
	ResetTime string  `json:"resetTime"`
}

type codexAuth struct {
	Tokens struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

type codexUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow struct {
			UsedPercent       float64 `json:"used_percent"`
			ResetAfterSeconds int     `json:"reset_after_seconds"`
		} `json:"primary_window"`
		SecondaryWindow struct {
			UsedPercent       float64 `json:"used_percent"`
			ResetAfterSeconds int     `json:"reset_after_seconds"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
}

type agyUsageResponse struct {
	Command struct {
		Data struct {
			Groups []struct {
				Name    string `json:"name"`
				Buckets []struct {
					Name              string  `json:"name"`
					Window            string  `json:"window"`
					RemainingFraction float64 `json:"remaining_fraction"`
					ResetTime         string  `json:"reset_time"`
				} `json:"buckets"`
			} `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

type App struct{}

func (a *App) GetVersion() string {
	return strings.TrimSpace(versionFile)
}

func (a *App) GetCodexUsage() CodexUsage {
	usage := CodexUsage{FetchedAt: time.Now().Format(time.RFC3339)}
	home, err := os.UserHomeDir()
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to get home directory: %v", err)
		return usage
	}

	authPath := filepath.Join(home, ".codex", "auth.json")
	authData, err := os.ReadFile(authPath)
	if err != nil {
		usage.Error = "Codex login information not found (~/.codex/auth.json)"
		return usage
	}
	var auth codexAuth
	if err := json.Unmarshal(authData, &auth); err != nil || auth.Tokens.AccessToken == "" {
		usage.Error = "Unable to read Codex access token"
		return usage
	}

	request, err := http.NewRequest(http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to create request: %v", err)
		return usage
	}
	request.Header.Set("Authorization", "Bearer "+auth.Tokens.AccessToken)
	response, err := httpClient.Do(request)
	if err != nil {
		usage.Error = fmt.Sprintf("Failed to fetch Codex usage: %v", err)
		return usage
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(response.Body)
		bodyStr := strings.TrimSpace(string(bodyBytes))
		if bodyStr != "" && len(bodyStr) < 150 {
			usage.Error = fmt.Sprintf("Codex request failed (HTTP %d): %s", response.StatusCode, bodyStr)
		} else {
			usage.Error = fmt.Sprintf("Codex usage request failed (HTTP %s)", response.Status)
		}
		return usage
	}

	var data codexUsageResponse
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		usage.Error = fmt.Sprintf("Unable to parse Codex usage response: %v", err)
		return usage
	}
	usage.Plan = data.PlanType
	usage.FiveHour = data.RateLimit.PrimaryWindow.UsedPercent
	usage.SevenDay = data.RateLimit.SecondaryWindow.UsedPercent
	usage.FiveHourIn = data.RateLimit.PrimaryWindow.ResetAfterSeconds
	usage.SevenDayIn = data.RateLimit.SecondaryWindow.ResetAfterSeconds
	return usage
}

func (a *App) GetAntigravityUsage() AntigravityUsage {
	usage := AntigravityUsage{FetchedAt: time.Now().Format(time.RFC3339)}

	agyPath, err := exec.LookPath("agy")
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			usage.Error = "agy command not found"
			return usage
		}
		agyPath = filepath.Join(home, "AppData", "Local", "agy", "bin", "agy.exe")
		if _, statErr := os.Stat(agyPath); statErr != nil {
			usage.Error = "agy command not found in PATH or AppData"
			return usage
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, agyPath, "-p", "/usage", "--output-format", "json", "--print-timeout", "30s")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			usage.Error = "agy usage request timed out (45s)"
		} else {
			errMsg := strings.TrimSpace(string(output))
			if errMsg != "" {
				usage.Error = fmt.Sprintf("agy failed: %s", errMsg)
			} else {
				usage.Error = fmt.Sprintf("Failed to fetch agy usage: %v", err)
			}
		}
		return usage
	}

	var response agyUsageResponse
	if err := json.Unmarshal(output, &response); err != nil {
		usage.Error = fmt.Sprintf("Unable to parse agy usage response: %v", err)
		return usage
	}

	for _, group := range response.Command.Data.Groups {
		parsedGroup := AntigravityUsageGroup{Name: group.Name}
		for _, bucket := range group.Buckets {
			parsedGroup.Buckets = append(parsedGroup.Buckets, AntigravityUsageBucket{
				Name:      bucket.Name,
				Window:    bucket.Window,
				Remaining: bucket.RemainingFraction * 100,
				ResetTime: bucket.ResetTime,
			})
		}
		usage.Groups = append(usage.Groups, parsedGroup)
	}
	if len(usage.Groups) == 0 {
		usage.Error = "No agy usage data found"
	}
	return usage
}

func init() {
	var err error
	frontendAssets, err = fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		panic("failed to initialize embedded frontend assets: " + err.Error())
	}
}

func main() {
	var window *application.WebviewWindow
	app := application.New(application.Options{
		Name: "AI Gauge",
		Icon: appIcon,
		Services: []application.Service{
			application.NewService(&App{}),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(frontendAssets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:      "com.aigauge.app",
			EncryptionKey: singleInstanceKey,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if window != nil {
					window.Show()
					window.Focus()
				}
			},
		},
	})

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "AI Gauge",
		Width:     300,
		Height:    500,
		Frameless: true,
		Windows: application.WindowsWindow{
			NonClientRegionSupport: true,
		},
	})
	showWindow := func() {
		window.Restore()
		window.Show()
		window.Focus()
	}
	placeTopRight := func() {
		screen := app.Screen.GetPrimary()
		if screen == nil {
			return
		}
		width, _ := window.Size()
		window.SetPosition(screen.WorkArea.X+screen.WorkArea.Width-width, screen.WorkArea.Y)
	}
	initialPlacement := true
	window.OnWindowEvent(events.Windows.WindowShow, func(_ *application.WindowEvent) {
		if initialPlacement {
			placeTopRight()
			initialPlacement = false
		}
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
		if initialPlacement {
			placeTopRight()
			initialPlacement = false
		}
	})
	clampWindow := func() {
		screen, err := window.GetScreen()
		if err != nil || screen == nil {
			return
		}
		x, y := window.Position()
		width, height := window.Size()
		work := screen.WorkArea
		newX, newY := x, y
		if width >= work.Width {
			newX = work.X
		} else if x < work.X {
			newX = work.X
		} else if x+width > work.X+work.Width {
			newX = work.X + work.Width - width
		}
		if height >= work.Height {
			newY = work.Y
		} else if y < work.Y {
			newY = work.Y
		} else if y+height > work.Y+work.Height {
			newY = work.Y + work.Height - height
		}
		if newX == x && newY == y {
			return
		}
		window.SetPosition(newX, newY)
	}
	window.OnWindowEvent(events.Windows.WindowEndMove, func(_ *application.WindowEvent) {
		clampWindow()
	})

	tray := app.SystemTray.New()
	tray.SetLabel("AI Gauge")
	tray.SetIcon(appIcon)
	tray.OnClick(func() {
		showWindow()
	})

	menu := app.NewMenu()
	menu.Add("Show").OnClick(func(_ *application.Context) {
		showWindow()
	})
	menu.Add("Exit").OnClick(func(_ *application.Context) {
		app.Quit()
	})
	tray.SetMenu(menu)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
