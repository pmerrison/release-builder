// Copyright Istio Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pkg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/howardjohn/istio-release/pkg/model"
	"github.com/howardjohn/istio-release/pkg/util"

	"istio.io/pkg/log"
)

// Sources will copy all dependencies require, pulling from Github if required, and set up the working tree.
// This includes locally tagging all git repos with the version being built, so that the right version is present in binaries.
func Sources(manifest model.Manifest) error {
	for _, dependency := range manifest.Dependencies {
		// Fetch the dependency
		if err := util.Clone(dependency, path.Join(manifest.SourceDir(), dependency.Repo)); err != nil {
			return fmt.Errorf("failed to resolve %+v: %v", dependency, err)
		}
		log.Infof("Resolved %v", dependency.Repo)

		// Also copy it to the working directory
		src := path.Join(manifest.SourceDir(), dependency.Repo)
		if err := util.CopyDir(src, manifest.RepoDir(dependency.Repo)); err != nil {
			return fmt.Errorf("failed to copy dependency %v to working directory: %v", dependency.Repo, err)
		}

		// Tag the repo. This allows the build process to look at the git tag for version information
		if err := TagRepo(manifest, manifest.RepoDir(dependency.Repo)); err != nil {
			return fmt.Errorf("failed to tag repo %v: %v", dependency.Repo, err)
		}
	}
	return nil
}

// The release expects a working directory with:
// * sources/ contains all of the sources to build from. These should not be modified
// * work/ initially contains all the sources, but may be modified during the build
// * out/ contains all final artifacts
func SetupWorkDir(dir string) error {
	if err := os.Mkdir(path.Join(dir, "sources"), 0750); err != nil {
		return fmt.Errorf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(dir, "work"), 0750); err != nil {
		return fmt.Errorf("failed to set up working directory: %v", err)
	}
	if err := os.Mkdir(path.Join(dir, "out"), 0750); err != nil {
		return fmt.Errorf("failed to set up working directory: %v", err)
	}
	return nil
}

// TagRepo tags a given git repo with the version from the manifest.
func TagRepo(manifest model.Manifest, repo string) error {
	headSha, err := GetSha(repo, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get HEAD SHA: %v", err)
	}
	currentTagSha, _ := GetSha(repo, manifest.Version)
	if currentTagSha != "" {
		if currentTagSha == headSha {
			log.Infof("Tag %v already exists, but points to the right place.", manifest.Version)
			return nil
		}
		return fmt.Errorf("tag %v already exists, retagging would move from %v to %v", manifest.Version, currentTagSha, headSha)
	}
	cmd := util.VerboseCommand("git", "tag", manifest.Version)
	cmd.Dir = repo
	return cmd.Run()
}

// GetSha returns the SHA for a given reference, or error if sha is not found
func GetSha(repo string, ref string) (string, error) {
	buf := bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Stdout = &buf
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// StandardizeManifest will convert a manifest to a fixed SHA, rather than a branch
// This allows outputting the exact version used after the build is complete
func StandardizeManifest(manifest *model.Manifest) error {
	for i, dep := range manifest.Dependencies {
		sha, err := GetSha(manifest.RepoDir(dep.Repo), "HEAD")
		if err != nil {
			return fmt.Errorf("failed to get SHA for %v: %v", dep.Repo, err)
		}
		dep.Sha = strings.TrimSpace(sha)
		dep.Branch = ""
		manifest.Dependencies[i] = dep
	}
	return nil
}