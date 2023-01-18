package main

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

const repoSpliter = "/"

type configuration struct {
	Config accessConfig `json:"access,omitempty"`
}

func (c *configuration) Validate() error {
	return c.Config.validate()
}

func (c *configuration) SetDefault() {}

type accessConfig struct {
	// Plugins is a map of repositories (eg "k/k") to lists of plugin names.
	RepoPlugins []map[string][]string `json:"repo_plugins,omitempty"`

	// Plugins is a list available plugins.
	Plugins []pluginConfig `json:"plugins,omitempty"`
}

func (a accessConfig) validate() error {
	ps := sets.NewString()
	for i := range a.Plugins {
		if err := a.Plugins[i].validate(); err != nil {
			return err
		}
		ps.Insert(a.Plugins[i].Name)
	}

	for _, item := range a.RepoPlugins {
		if v := sets.NewString(item["plugins"]...).Difference(ps); v.Len() != 0 {
			return fmt.Errorf(
				"unknown plugins(%s) are set", strings.Join(v.UnsortedList(), ", "),
			)
		}
	}

	return nil
}

type eventsDemux map[string][]string

func updateDemux(p *pluginConfig, d eventsDemux) {
	endpoint := p.Endpoint

	for _, e := range p.Events {
		if es, ok := d[e]; ok {
			d[e] = append(es, endpoint)
		} else {
			d[e] = []string{endpoint}
		}
	}
}

func orgOfRepo(repo string) string {
	if strings.Contains(repo, repoSpliter) {
		return strings.Split(repo, repoSpliter)[0]
	}

	return ""
}

func (a accessConfig) getDemux() map[string]eventsDemux {
	plugins := make(map[string]int)
	for i := range a.Plugins {
		plugins[a.Plugins[i].Name] = i
	}

	r := make(map[string]eventsDemux)
	orgPlugins := a.getOrgPlugins()

	for _, rps := range a.RepoPlugins {
		for _, repo := range rps["repos"] {
			events, ok := r[repo]
			if !ok {
				events = make(eventsDemux)
				r[repo] = events
			}

			ps := a.appendOrgPlugins(orgPlugins, repo, rps["plugins"])

			for _, p := range ps {
				if i, ok := plugins[p]; ok {
					updateDemux(&a.Plugins[i], events)
				}
			}
		}
	}

	return r
}

func (a accessConfig) getOrgPlugins() map[string][]string {
	org := make(map[string][]string)

	for _, rps := range a.RepoPlugins {
		for _, rp := range rps["repos"] {
			if !strings.Contains(rp, repoSpliter) {
				org[rp] = rps["plugins"]
			}
		}
	}

	return org
}

func (a accessConfig) appendOrgPlugins(cps map[string][]string, repo string, ps []string) []string {
	org := orgOfRepo(repo)
	if org == "" {
		return ps
	}

	if p, ok := cps[org]; ok {
		ps = append(ps, p...)
	}

	return ps
}

type pluginConfig struct {
	// Name of the plugin.
	Name string `json:"name" required:"true"`

	// Endpoint is the location of the plugin.
	Endpoint string `json:"endpoint" required:"true"`

	// Events are the events that this plugin can handle and should be forward to it.
	// If no events are specified, everything is sent.
	Events []string `json:"events,omitempty"`
}

func (p pluginConfig) validate() error {
	if p.Name == "" {
		return fmt.Errorf("missing name")
	}

	if p.Endpoint == "" {
		return fmt.Errorf("missing endpoint")
	}

	// TODO validate the value of p.Endpoint
	return nil
}
