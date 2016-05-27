package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/grsmv/goweek"
	"github.com/roessland/gotoggl"
	"log"
	"regexp"
	"time"
)

var _ = fmt.Printf

// A streak contains a calendar with info about which days were successful.
type Streak struct {
	// Loaded from config
	Name         string
	Description  string
	WorkspaceId  int
	ProjectId    int
	RegexpString string `toml:"regexp"`
	MinutesMin   int

	// Modified by Init()
	Regexp *regexp.Regexp
}

// Init compiles regex and creates a calendar
func (s *Streak) Init() {
	s.Regexp = regexp.MustCompile(s.RegexpString)
}

// Match checks if a time entry belongs to a streak
func (s *Streak) Match(te *gotoggl.TimeEntry) bool {
	if s.WorkspaceId != 0 && s.WorkspaceId != te.WorkspaceId {
		return false
	}
	if s.ProjectId != 0 && s.ProjectId != te.ProjectId {
		return false
	}
	if !s.Regexp.MatchString(te.Description) {
		return false
	}
	return true
}

func (s *Streak) Add(te *gotoggl.TimeEntry) {

}

type Config struct {
	ApiKey             string
	Timezone           string
	WeeksBeforeCurrent int
	WeeksAfterCurrent  int
	Streaks            map[string]Streak

	// Initialized by Init()
	Location *time.Location
}

// Init loads the timezone location
func (conf *Config) Init() {
	var err error
	conf.Location, err = time.LoadLocation(conf.Timezone)
	if err != nil {
		log.Fatalf("Couldn't load timezone %v: %v\n", conf.Timezone, err)
	}
}

func main() {
	var conf Config
	if _, err := toml.DecodeFile("config.toml", &conf); err != nil {
		log.Fatal(err)
	}
	log.Print("config.toml loaded")

	// Create a client and get current user
	toggl := gotoggl.NewClient(conf.ApiKey)
	me, err := toggl.Me.Get()
	if err != nil {
		log.Fatalf("Couldn't get current user: %v\n", err)
	}

	// Find user timezone if timezone not given by config
	if conf.Timezone == "" {
		conf.Timezone = me.Timezone
		log.Printf("No timezone in config; using Toggl user timezone %v.\n", me.Timezone)
	}
	conf.Init()

	// Set up streaks from config
	streaks := make(map[string]*Streak)
	for name, streak := range conf.Streaks {
		log.Printf("Loaded streak %v\n", name)
		streaks[name] = &streak
		streaks[name].Init()
	}

	// Find the start of the streak calendar by going
	// `config.WeeksBeforeCurrent` weeks back, and finding the datetime when
	// Monday started. Uses the user timezone. E.g., if the user is UTC+2, then their week begins
	// at Sunday 22:00 UTC or Monday 00:00 UTC. Toggl expects UTC, so we
	// convert back using the timezone offset. (Is there a better way to do
	// this?)
	nowLocal := time.Now().In(conf.Location).Add(time.Duration(24*2+8) * time.Hour)
	yr, wk := nowLocal.Add(time.Duration(-7*24*conf.WeeksBeforeCurrent) * time.Hour).ISOWeek()
	week, _ := goweek.NewWeek(yr, wk)
	monday := week.Days[0]
	_, tzOffset := nowLocal.Zone()
	monday = monday.Add(-time.Duration(tzOffset) * time.Second)

	entries, err := toggl.TimeEntries.Range(monday, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	// 1000 is the limit for returned entries. If the limit is reached, this
	// means that the most recent entries have been skipped, and only older
	// entries returned. This can be circumvented using Toggl's paginated
	// reports API.
	if len(entries) == 1000 {
		log.Fatalf("Entry limit reached (1000 entries). Decrease conf.WeeksBeforeCurrent.")
	}

	// Add entries to matching streaks
	for _, entry := range entries {
		for _, streak := range streaks {
			if streak.Match(&entry) {
				streak.Add(&entry)
			}
		}
	}
	/*
		if entry.Description == "Anki" {
			localStart := entry.Start.In(conf.Location)
			fmt.Printf("At %v you studied Anki for %v minutes\n",
				localStart.Format("2006-01-02 15:04"),
				entry.Duration.Minutes())
		}
	*/
}
