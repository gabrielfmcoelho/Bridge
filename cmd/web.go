package cmd

import (
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/api"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/spf13/cobra"
)

var (
	webPort int
	webOpen bool
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web UI and API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := database.Open(sshcmConfigDir())
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer db.Close()

		apiRouter := api.NewRouter(db, configPath)

		mux := http.NewServeMux()

		// API routes
		mux.Handle("/api/", apiRouter)

		// Serve embedded static frontend (Next.js export) at /
		staticFS, err := fs.Sub(staticFS, "static")
		if err != nil {
			// No static directory embedded yet — serve a placeholder.
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/") {
					return
				}
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `<!DOCTYPE html><html><body style="background:#111;color:#eee;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh">
					<div style="text-align:center"><h1>SSHCM API</h1><p>Frontend not built yet. Run <code>make build-frontend</code> first.</p>
					<p style="margin-top:1em">API available at <a href="/api/auth/status" style="color:#60a5fa">/api/auth/status</a></p></div></body></html>`)
			})
		} else {
			fileServer := http.FileServer(http.FS(staticFS))
			mux.Handle("/", fileServer)
		}

		addr := fmt.Sprintf(":%d", webPort)
		url := fmt.Sprintf("http://localhost:%d", webPort)

		if webOpen {
			go openBrowser(url)
		}

		fmt.Printf("Starting sshcm at %s\n", url)
		fmt.Println("Press Ctrl+C to stop")

		// http.Server with explicit timeouts. WriteTimeout intentionally
		// left unset because long-running SSH operations (TestCapture
		// runs many sequential commands) can legitimately exceed any
		// fixed deadline. ReadHeaderTimeout caps slowloris-style stalls;
		// IdleTimeout closes leaking keep-alive sockets.
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		return srv.ListenAndServe()
	},
}

func init() {
	webCmd.Flags().IntVar(&webPort, "port", 8080, "port to listen on")
	webCmd.Flags().BoolVar(&webOpen, "open", false, "open browser automatically")
	rootCmd.AddCommand(webCmd)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		cmd.Start()
	}
}
