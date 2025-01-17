// apparmor.d - Full set of apparmor profiles
// Copyright (C) 2023 Alexandre Pujol <alexandre@pujol.io>
// SPDX-License-Identifier: GPL-2.0-only

package integration

import (
	"io"
	"net/http"
	"strings"

	"github.com/arduino/go-paths-helper"
	"github.com/pkg/errors"
	"github.com/roddhjav/apparmor.d/pkg/util"
)

var (
	Ignore    []string          // Do not run some scenarios
	Arguments map[string]string // Common arguments used across all scenarios
)

type Tldr struct {
	Url    string      // Tldr download url
	Dir    *paths.Path // Tldr cache directory
	Ignore []string    // List of ignored software
}

func NewTldr(dir *paths.Path) Tldr {
	return Tldr{
		Url: "https://github.com/tldr-pages/tldr/archive/refs/heads/main.tar.gz",
		Dir: dir,
	}
}

// Download and extract the tldr pages into the cache directory
func (t Tldr) Download() error {
	gzPath := t.Dir.Parent().Join("tldr.tar.gz")
	if !gzPath.Exist() {
		resp, err := http.Get(t.Url)
		if err != nil {
			return errors.Wrapf(err, "downloading %s", t.Url)
		}
		defer resp.Body.Close()

		out, err := gzPath.Create()
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			return err
		}
	}

	pages := []string{"tldr-main/pages/linux", "tldr-main/pages/common"}
	if err := util.ExtratTo(gzPath, t.Dir, pages); err != nil {
		return err
	}
	return nil
}

// Parse the tldr pages and return a list of scenarios
func (t Tldr) Parse(profiles paths.PathList) (*TestSuite, error) {
	testSuite := NewTestSuite()
	files, _ := t.Dir.ReadDirRecursiveFiltered(nil, paths.FilterOutDirectories())
	for _, path := range files {
		content, err := path.ReadFile()
		if err != nil {
			return nil, err
		}
		raw := string(content)
		scenario := &Scenario{
			Name:      strings.TrimSuffix(path.Base(), ".md"),
			Profiled:  false,
			Root:      false,
			Arguments: map[string]string{},
			Tests:     []Test{},
		}
		scenario.Profiled = scenario.hasProfile(profiles)
		if strings.Contains(raw, "sudo") {
			scenario.Root = true
		}
		rawTests := strings.Split(raw, "\n-")[1:]
		for _, test := range rawTests {
			res := strings.Split(test, "\n")
			dsc := strings.ReplaceAll(strings.Trim(res[0], " "), ":", "")
			cmd := strings.Trim(strings.Trim(res[2], "`"), " ")
			if scenario.Root {
				cmd = strings.ReplaceAll(cmd, "sudo ", "")
			}
			scenario.Tests = append(scenario.Tests, Test{
				Description: dsc,
				Command:     cmd,
			})
		}
		testSuite.Scenarios = append(testSuite.Scenarios, *scenario)
	}
	return testSuite, nil
}
