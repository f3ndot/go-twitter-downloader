package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	twitterscraper "github.com/imperatrona/twitter-scraper"
	"os"
)

type TweetEntry struct {
	ID int `json:"id"`
}

type AuthCookies struct {
	AuthToken string `json:"auth_token"`
	CT0       string `json:"ct0"`
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <username>\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
}

func main() {
	// Define flags
	outputFile := flag.String("output", "<username>.json", "Where the tweets should be saved")
	maxTweets := flag.Int("number", 3200, "Max number of tweets to download")
	includeRetweets := flag.Bool("retweets", false, "Include retweets/RTs in download")
	verbose := flag.Bool("verbose", false, "Enable verbose mode")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: username is required")
		flag.Usage()
		os.Exit(1)
	}
	username := flag.Arg(0)

	if *outputFile == "<username>.json" {
		*outputFile = fmt.Sprintf("%s.json", username)
	}

	fmt.Printf("Downloading tweets for: %s, output: %s, verbose: %t\n", username, *outputFile, *verbose)

	scraper := twitterscraper.New()

	// Deserialize from JSON
	var authCookies AuthCookies
	cf, _ := os.Open("cookies.json")
	err := json.NewDecoder(cf).Decode(&authCookies)
	if err != nil {
		panic("unable to open cookies.json")
	}
	err = cf.Close()
	if err != nil {
		panic("unable to close cookies.json")
	}

	scraper.SetAuthToken(twitterscraper.AuthToken{Token: authCookies.AuthToken, CSRFToken: authCookies.CT0})
	if !scraper.IsLoggedIn() {
		panic("Invalid auth cookies")
	}

	f, err := os.OpenFile(*outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Errorf("unable to open output file: %w", err))
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	_, err = f.WriteString("{ \"tweets\": [\n")
	if err != nil {
		panic(fmt.Errorf("unable to start writing to output file: %w", err))
	}
	encoder := json.NewEncoder(f)
	stdoutEncoder := json.NewEncoder(os.Stdout)
	isFirstTweet := true
	for tweet := range scraper.GetTweets(context.Background(), username, *maxTweets) {

		if tweet.Error != nil {
			_, err := fmt.Fprintln(os.Stderr, fmt.Errorf("ERROR: %w", tweet.Error))
			if err != nil {
				panic(fmt.Errorf("unable to print error to stderr: %w", tweet.Error))
			}
		}
		if *includeRetweets == false && tweet.IsRetweet {
			fmt.Printf("Skipping RT %s...\n", tweet.ID)
			continue
		}
		if *verbose {
			stdoutEncoder.Encode(tweet.Tweet)
		} else {
			fmt.Printf("Downloading %s...\n", tweet.ID)
		}
		if isFirstTweet {
			isFirstTweet = false
		} else {
			_, err = f.WriteString(",")
			if err != nil {
				panic(fmt.Errorf("unable write preceding comma: %w", err))
			}
		}
		if err := encoder.Encode(tweet.Tweet); err != nil {
			_, err := fmt.Fprintln(os.Stderr, fmt.Errorf("ERROR encoding: %w", tweet.Error))
			if err != nil {
				panic(fmt.Errorf("unable to print error to stderr: %w", tweet.Error))
			}
			return
		}
	}
	_, err = f.WriteString("] }")
	if err != nil {
		panic(fmt.Errorf("unable to finish writing to output file: %w", err))
	}
}
