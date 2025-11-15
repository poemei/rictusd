package agents

import (
	"archive/tar"
	"compress/gzip"
	"context"
	//"encoding/json"
	//"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type GoBuildAgent struct{}

func (GoBuildAgent) Name() string { return "go" }
func (GoBuildAgent) Ping(ctx context.Context) error { return nil }

func (GoBuildAgent) Run(ctx context.Context, op string, payload map[string]any) (map[string]any, error) {
	workdir, _ := payload["workdir"].(string)
	artdir,  _ := payload["artifact_dir"].(string)
	if workdir == "" || artdir == "" {
		return map[string]any{"status":"ERROR","error":"workdir and artifact_dir are required"}, nil
	}
	steps := []string{"vet","build","test","package"}
	if v, ok := payload["steps"].([]any); ok && len(v) > 0 {
		steps = nil; for _, x := range v { if s,ok:=x.(string); ok { steps = append(steps, s) } }
	}
	_ = os.MkdirAll(artdir, 0o755)
	env := append(os.Environ(), "CGO_ENABLED=0")

	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = workdir
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	for _, s := range steps {
		switch s {
		case "vet":
			if err := run("vet","./..."); err != nil {
				return map[string]any{"status":"FAILED_POLICY","violations":[]any{map[string]any{"code":"go.vet","detail":err.Error()}}}, nil
			}
		case "build":
			if err := run("build","./..."); err != nil {
				return map[string]any{"status":"ERROR","error":"build failed: "+err.Error()}, nil
			}
		case "test":
			if err := run("test","-cover","./..."); err != nil {
				return map[string]any{"status":"FAILED_POLICY","violations":[]any{map[string]any{"code":"go.tests","detail":err.Error()}}}, nil
			}
		case "package":
			art := filepath.Join(artdir, "artifact.tar.gz")
			if err := tarGz(workdir, art); err != nil {
				return map[string]any{"status":"ERROR","error":"package failed: "+err.Error()}, nil
			}
		}
	}

	return map[string]any{
		"status":"SUCCESS",
		"artifacts":[]any{ filepath.Join(artdir, "artifact.tar.gz") },
		"metrics":map[string]any{ "completed_at": time.Now().UTC().Format(time.RFC3339), "cgo_enabled": 0 },
	}, nil
}

func tarGz(src, dst string) error {
	f, err := os.Create(dst); if err != nil { return err }
	defer f.Close()
	gz := gzip.NewWriter(f); defer gz.Close()
	tw := tar.NewWriter(gz); defer tw.Close()
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil { return err }
		if filepath.Clean(p) == filepath.Clean(dst) { return nil }
		h, err := tar.FileInfoHeader(info, ""); if err != nil { return err }
		rel, err := filepath.Rel(src, p); if err != nil { return err }
		h.Name = rel
		if err := tw.WriteHeader(h); err != nil { return err }
		if info.Mode().IsRegular() {
			r, err := os.Open(p); if err != nil { return err }
			_, err = io.Copy(tw, r); _ = r.Close(); if err != nil { return err }
		}
		return nil
	})
}

