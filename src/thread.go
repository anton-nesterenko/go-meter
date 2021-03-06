package main

import (
	"strconv"
	"strings"
	"net/http"
	"name/away/settings"
	"time"
	"regexp"
	"bytes"
	"io/ioutil"
)

type Status struct {
	IsError bool
	IsWarning bool
	IsSuccess bool
	Duration *time.Duration
	Size int64
	IsFinished bool
	Error *error
	FinishedAt *time.Time
	StartedAt *time.Time
}

func checkStatus(levels settings.Levels, resp *http.Response, times time.Duration, status *Status){
	status.Duration = &times
	status.Size = resp.Request.ContentLength
	milliseconds := int64(times.Seconds() * 1000)
	if levels.Success != nil {
		if levels.Success.Timeout == 0 || int64(levels.Success.Timeout) >= milliseconds {
			for _, st := range levels.Success.Codes {
				if st == resp.StatusCode {
					status.IsSuccess = true
					return
				}
			}
		}
	}
	if levels.Warning != nil && !status.IsSuccess {
		if levels.Warning.Timeout == 0 || int64(levels.Warning.Timeout) >= milliseconds {
			for _, st := range levels.Warning.Codes {
				if st == resp.StatusCode {
					status.IsWarning = true
					return
				}
			}
		}
	}
	if levels.Error != nil && !status.IsSuccess && !status.IsWarning {
		if levels.Error.Timeout == 0 || int64(levels.Error.Timeout) >= milliseconds {
			for _, st := range levels.Error.Codes {
				if st == resp.StatusCode {
					status.IsError = true
					return
				}
			}
		}
	} else if levels.Error == nil && !status.IsSuccess && !status.IsWarning {
		status.IsError = true
	}

}

func StartThread(setts *settings.Settings, source *Source, c chan *Status){
	iteration := setts.Threads.Iteration
	header := map[string]string{}
	for _, s := range setts.Request.Headers {
		keyValue := regexp.MustCompile("=").Split(s, -1)
		header[keyValue[0]] = keyValue[1]
	}

	sourceLen := len(*source)

	url := setts.Remote.Protocol + "://" + setts.Remote.Host + ":" + strconv.Itoa(setts.Remote.Port) + setts.Request.Uri
	if iteration < 0 {
		iteration = sourceLen
	}
	index := -1
	for ;iteration > 0; iteration-- {
		status := &Status{false, false, false, nil, 0, false, nil, nil, nil}
		index++
		if index >= sourceLen {
			if setts.Request.Source.RestartOnEOF {
				index = 0
			} else {
				index--
			}
		}
		var s *bytes.Buffer
		if strings.ToLower(setts.Request.Method) != "get" {
			s = bytes.NewBuffer((*source)[index])
		}
		req, err := http.NewRequest(setts.Request.Method, url, s); if err != nil {
			status.Error = &err
			status.IsError = true
			c <- status
			break
		}
		for k,v := range header {
			req.Header.Set(k,v)
		}
		startTime := time.Now()
		res, err := http.DefaultClient.Do(req); if err != nil {
			status.Error = &err
			status.IsError = true
			c <- status
			break
		}
		endTime := time.Now()
		status.FinishedAt = &endTime
		status.StartedAt = &startTime
		diff := endTime.Sub(startTime)
		checkStatus(setts.Levels, res, diff, status)
		ioutil.ReadAll(res.Body)
		res.Body.Close()
		c <- status

		if setts.Threads.Delay > 0 {
			sleep := time.Duration(setts.Threads.Delay)
			time.Sleep(time.Millisecond * sleep)
		}
	}
	status := &Status{false, false, false, nil, 0, true, nil, nil, nil}
	c <- status
}

