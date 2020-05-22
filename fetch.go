package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func parsePOM(bytes []byte) *Project {
	var project Project
	xml.Unmarshal(bytes, &project)
	return &project
}

func parseMeta(bytes []byte) *Metadata {
	var meta Metadata
	xml.Unmarshal(bytes, &meta)
	return &meta
}

func readPOM(path string) (*Project, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return parsePOM(bytes), nil
}

/* TODO implement a timeout */
func fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(
			fmt.Sprintf("failed to fetch with: %d", resp.StatusCode))
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func repos() []string {
	return []string{
		"https://repo.maven.apache.org/maven2",
		"https://dl.google.com/dl/android/maven2",
		"https://repository.sonatype.org/content/groups/sonatype-public-grid",
		"https://plugins.gradle.org/m2",
		"https://maven.java.net/content/repositories/releases",
		"https://jcenter.bintray.com",
		"https://jitpack.io",
		"https://repo1.maven.org/maven2",
	}
}

type FetcherResult struct {
	url  string
	repo string
	data []byte
}

type FetcherJob struct {
	result chan FetcherResult
	path   string
	repo   string
}

type FetcherPool struct {
	limit int
	queue chan FetcherJob
}

func NewFetcherPool(l int) FetcherPool {
	f := FetcherPool{
		limit: l,
		queue: make(chan FetcherJob, l),
	}
	/* start workers */
	for i := 0; i < f.limit; i++ {
		go f.Worker()
	}
	return f
}

func (p *FetcherPool) TryRepo(repo, path string) *FetcherResult {
	url := repo + "/" + path
	data, err := fetch(url)
	if err == nil {
		return &FetcherResult{url, repo, data}
	} else {
		fmt.Sprintln(os.Stderr, "Failed to fetch:", err)
		return nil
	}
}

func (p *FetcherPool) TryRepos(job FetcherJob) {
	/* repo can be provided in the job */
	if job.repo != "" {
		rval := p.TryRepo(job.repo, job.path)
		if rval != nil {
			job.result <- *rval
			return
		}
	} else {
		for _, repo := range repos() {
			rval := p.TryRepo(repo, job.path)
			if rval != nil {
				job.result <- *rval
				return
			}
		}
	}
	job.result <- FetcherResult{}
}

func (p *FetcherPool) Worker() {
	for job := range p.queue {
		p.TryRepos(job)
	}
}