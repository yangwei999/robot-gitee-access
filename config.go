package main

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	pluginsGroupNamePrefix = "pg-"
	repoGroupNamePrefix    = "rg-"
)

type configuration struct {
	Config accessConfig `json:"access,omitempty"`
}

func (c *configuration) Validate() error {
	return c.Config.validate()
}

func (c *configuration) SetDefault() {}

type accessConfig struct {
	// Plugins is a map of repositories (eg "k/k") to lists of plugin names.
	RepoPlugins map[string][]string `json:"repo_plugins,omitempty"`

	// RepoGroup is a group of repos.
	RepoGroup map[string][]string `json:"repo_group,omitempty"`

	// PluginsGroup is a group of plugins.
	PluginsGroup map[string][]string `json:"plugins_group,omitempty"`

	// Plugins is a list available plugins.
	Plugins []pluginConfig `json:"plugins,omitempty"`
}

func (a accessConfig) validate() error {
	pluginsTotal := sets.NewString()
	for i := range a.Plugins {
		if err := a.Plugins[i].validate(); err != nil {
			return err
		}
		pluginsTotal.Insert(a.Plugins[i].Name)
	}

	groupsTotal := sets.NewString()
	for k := range a.PluginsGroup {
		if !a.isPluginsGroupName(k) {
			return fmt.Errorf("%s:plugins group name must be start with %s", k, pluginsGroupNamePrefix)
		}
		groupsTotal.Insert(k)
	}

	for k := range a.RepoGroup {
		if !a.isRepoGroupName(k) {
			return fmt.Errorf("%s:repo group name must be start with %s", k, repoGroupNamePrefix)
		}
	}

	for k, item := range a.RepoPlugins {
		pluginsGroup, plugins := a.separateItem(item)

		if v := pluginsGroup.Difference(groupsTotal); v.Len() != 0 {
			return fmt.Errorf(
				"%s: unknown plugins_group(%s) are set", k,
				strings.Join(v.UnsortedList(), ", "),
			)
		}

		// validate the plugins specified in group
		for gn, _ := range pluginsGroup {
			plugins.Insert(a.PluginsGroup[gn]...)
		}

		if v := plugins.Difference(pluginsTotal); v.Len() != 0 {
			return fmt.Errorf(
				"%s: unknown plugins(%s) are set", k,
				strings.Join(v.UnsortedList(), ", "),
			)
		}
	}

	return nil
}

func (a accessConfig) separateItem(item []string) (groups, plugins sets.String) {
	groups = sets.NewString()
	plugins = sets.NewString()
	for _, v := range item {
		if a.isPluginsGroupName(v) {
			groups.Insert(v)
		} else {
			plugins.Insert(v)
		}
	}

	return
}

func (a accessConfig) isPluginsGroupName(name string) bool {
	return strings.HasPrefix(name, pluginsGroupNamePrefix)
}

func (a accessConfig) isRepoGroupName(name string) bool {
	return strings.HasPrefix(name, repoGroupNamePrefix)
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
	spliter := "/"
	if strings.Contains(repo, spliter) {
		return strings.Split(repo, spliter)[0]
	}
	return ""
}

func (a accessConfig) getDemux() map[string]eventsDemux {
	plugins := make(map[string]int)
	for i := range a.Plugins {
		plugins[a.Plugins[i].Name] = i
	}

	rp := a.changeRepoGroup()

	r := make(map[string]eventsDemux)
	for k, ps := range rp {
		events, ok := r[k]
		if !ok {
			events = make(eventsDemux)
			r[k] = events
		}

		// inherit the config of org if k is a repo.
		if org := orgOfRepo(k); org != "" {
			ps = append(ps, rp[org]...)
		}

		nps := a.changePluginsGroup(ps)

		for _, p := range nps {
			if i, ok := plugins[p]; ok {
				updateDemux(&a.Plugins[i], events)
			}
		}
	}

	return r
}

// changeRepoGroup change repo group name to repo list.
func (a accessConfig) changeRepoGroup() eventsDemux {
	nrp := make(eventsDemux)
	for k, ps := range a.RepoPlugins {
		if a.isRepoGroupName(k) {
			for _, v := range a.RepoGroup[k] {
				nrp[v] = append(nrp[v], ps...)
			}
		} else {
			nrp[k] = append(nrp[k], ps...)
		}
	}

	return nrp
}

// changePluginsGroup change plugins group name to plugins list,
// and ensure all elements are unique.
func (a accessConfig) changePluginsGroup(ps []string) []string {
	nps := sets.NewString()
	for _, p := range ps {
		if a.isPluginsGroupName(p) {
			nps.Insert(a.PluginsGroup[p]...)
		} else {
			nps.Insert(p)
		}
	}

	return nps.UnsortedList()
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
