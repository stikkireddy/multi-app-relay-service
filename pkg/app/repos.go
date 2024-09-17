package app

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"os"
	"path/filepath"
)

type GitRepo struct {
	Url       string `json:"url" yaml:"url"`
	Hash      string `json:"hash" yaml:"hash"`
	TargetDir string `json:"targetDir" yaml:"targetDir"`
}

func (gr *GitRepo) Clean() error {
	dir := gr.TargetDir
	if filepath.IsAbs(dir) {
		dir = filepath.Join(".", dir)
	}
	return os.RemoveAll(dir)
}

func (gr *GitRepo) Clone() error {
	dir := gr.TargetDir
	if filepath.IsAbs(dir) {
		dir = filepath.Join(".", dir)
	}
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL: gr.Url,
	})
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = wt.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(gr.Hash),
	})
	if err != nil {
		return err
	}
	return err
}
