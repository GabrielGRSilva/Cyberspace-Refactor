// Command cyberspace launches the unified Cyberspace Refactor desktop suite
// (Cyberspace Terminal, Matrix Construction Set, Cyberdeck Designer, and
// Decker Biomonitor).
package main

import (
	"flag"
	"fmt"
	"os"

	gioapp "gioui.org/app"

	capp "cyberspace-refactor/internal/app"
	"cyberspace-refactor/internal/engine"
	"cyberspace-refactor/internal/ui"
)

func main() {
	headless := flag.Bool("headless-check", false, "Run a self-test of workspace assets and Matrix engine, then exit")
	flag.Parse()

	ws, err := capp.Locate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cyberspace Refactor:", err)
		os.Exit(1)
	}

	if *headless {
		if err := runSelfCheck(ws); err != nil {
			fmt.Fprintln(os.Stderr, "Self-check failed:", err)
			os.Exit(1)
		}
		fmt.Println("Cyberspace Refactor", ui.Version, "headless self-check OK.")
		return
	}

	go func() {
		if err := ui.Run(ws); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}()
	gioapp.Main()
}

func runSelfCheck(ws *capp.Workspace) error {
	m, _, err := ws.LoadMatrix("DATA/FIRSTRUN.LTG")
	if err != nil {
		return err
	}
	deck, _, err := ws.LoadDeck("DATA/FUCHI4_2.DEK")
	if err != nil {
		return err
	}
	decker, _, err := ws.LoadDecker("DATA/HEAVYHIT.NPC")
	if err != nil {
		return err
	}
	sess, err := engine.New(engine.Config{
		Matrix: m,
		Deck:   deck,
		Decker: decker,
		Loader: ws.LinkLoader("DATA"),
	})
	if err != nil {
		return err
	}
	return sess.Wait()
}
