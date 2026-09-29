package gitmgr

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitHttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

type GitService struct{}

func New() *GitService {
	return &GitService{}
}

func (g *GitService) Clone(url, targetPath, branch, token string) error {
	opts := &git.CloneOptions{
		URL:      url,
		Progress: nil,
	}

	if branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		opts.SingleBranch = true
	}

	if token != "" {
		opts.Auth = &gitHttp.BasicAuth{
			Username: "oauth2",
			Password: token,
		}
	}

	_, err := git.PlainClone(targetPath, false, opts)
	return err
}

func (g *GitService) Pull(repoPath, token string) (string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	w, err := r.Worktree()
	if err != nil {
		return "", err
	}

	opts := &git.PullOptions{
		RemoteName: "origin",
	}

	if token != "" {
		opts.Auth = &gitHttp.BasicAuth{
			Username: "oauth2",
			Password: token,
		}
	}

	err = w.Pull(opts)
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up to date", nil
		}
		return "", err
	}

	ref, err := r.Head()
	if err != nil {
		return "Updated", nil
	}
	return fmt.Sprintf("Updated to %s", ref.Hash().String()[:7]), nil
}

func (g *GitService) CommitAndPush(repoPath, message, authorName, authorEmail, token string) error {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Add all changes
	if _, err := w.Add("."); err != nil {
		return err
	}

	if authorName == "" {
		authorName = "ActaCron User"
	}
	if authorEmail == "" {
		authorEmail = "user@actacron.local"
	}
	if message == "" {
		message = "Update scripts via ActaCron"
	}

	commit, err := w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  authorName,
			Email: authorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return err
	}

	// Push if remote exists
	remotes, err := r.Remotes()
	if err == nil && len(remotes) > 0 {
		pushOpts := &git.PushOptions{}
		if token != "" {
			pushOpts.Auth = &gitHttp.BasicAuth{
				Username: "oauth2",
				Password: token,
			}
		}
		if pushErr := r.Push(pushOpts); pushErr != nil && !errors.Is(pushErr, git.NoErrAlreadyUpToDate) {
			return fmt.Errorf("committed (%s) but push failed: %w", commit.String()[:7], pushErr)
		}
	}

	return nil
}

func (g *GitService) GetStatus(repoPath string) (string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "error", err
	}

	w, err := r.Worktree()
	if err != nil {
		return "error", err
	}

	status, err := w.Status()
	if err != nil {
		return "error", err
	}

	if !status.IsClean() {
		return "modified", nil
	}

	return "clean", nil
}
