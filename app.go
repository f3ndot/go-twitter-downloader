package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	twitterscraper "github.com/imperatrona/twitter-scraper"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type TweetEntry struct {
	ID int `json:"id"`
}

type AuthCookies struct {
	AuthToken string `json:"auth_token"`
	CT0       string `json:"ct0"`
}

// Google Sheets ID and range
var spreadsheetID string

const batchSize = 50

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
	flag.StringVar(&spreadsheetID, "spreadsheet-id", "", "The Google Sheet ID to upload results to")
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

	fmt.Printf("Downloading tweets for: %s, output: %s, verbose: %t, Google Sheet: %s\n", username, *outputFile, *verbose, spreadsheetID)

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

	credentialsFile := "service_account.json"
	data, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		log.Fatalf("Failed to read credentials file: %v", err)
	}
	config, err := google.CredentialsFromJSON(context.Background(), data, sheets.SpreadsheetsScope)
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	// Create Sheets service
	srv, err := sheets.NewService(context.Background(), option.WithCredentials(config))
	if err != nil {
		log.Fatalf("Failed to create Sheets service: %v", err)
	}
	_, err = f.WriteString("{ \"tweets\": [\n")
	if err != nil {
		panic(fmt.Errorf("unable to start writing to output file: %w", err))
	}
	encoder := json.NewEncoder(f)
	stdoutEncoder := json.NewEncoder(os.Stdout)
	isFirstTweet := true
	downloadedTweets := 0
	var spreadsheetDataBuf [][]interface{}
	if spreadsheetID != "" {
		fmt.Printf("Clearing data from spreadsheet %s..\n", spreadsheetID)
		_, err := srv.Spreadsheets.Values.Clear(spreadsheetID, "Sheet1", &sheets.ClearValuesRequest{}).Do()
		if err != nil {
			panic(fmt.Errorf("unable to clear Google Sheet: %w", err))
		}
		fmt.Printf("Setting headers for spreadsheet %s...\n", spreadsheetID)
		_, err = srv.Spreadsheets.Values.Append(spreadsheetID, "Sheet1!A1", &sheets.ValueRange{
			Values: [][]interface{}{
				{"Date/Time (ET)", "Link", "Text", "IsQuoted", "IsPin", "IsReply", "IsRetweet", "IsSelfThread", "Views", "Likes", "Retweets", "Replies"},
			},
		}).ValueInputOption("USER_ENTERED").Do()
		if err != nil {
			panic(fmt.Errorf("unable to write headers to Google Sheet: %w", err))
		}
		fmt.Printf("Setting up spreadsheet properties %s...\n", spreadsheetID)
		requests := []*sheets.Request{
			{
				UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
					Properties: &sheets.SheetProperties{
						SheetId: 0, // Change this if needed (0 is usually the first sheet)
						GridProperties: &sheets.GridProperties{
							FrozenRowCount: 1, // Freeze the first row
						},
					},
					Fields: "gridProperties.frozenRowCount",
				},
			},
			{
				UpdateSpreadsheetProperties: &sheets.UpdateSpreadsheetPropertiesRequest{
					Properties: &sheets.SpreadsheetProperties{
						Title: fmt.Sprintf("Downloaded Tweets - %s", username),
					},
					Fields: "title",
				},
			},
			// TODO: Put the header's append into here
		}
		// Send batch update request
		batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}
		_, err = srv.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdateRequest).Do()
		if err != nil {
			log.Fatalf("Failed to freeze the first row: %v", err)
		}
	}
	tz, err := time.LoadLocation("America/Toronto")
	if err != nil {
		panic(err)
	}
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
		if spreadsheetID != "" {
			spreadsheetDataBuf = append(spreadsheetDataBuf, []interface{}{
				tweet.TimeParsed.In(tz).Format(time.DateTime),
				tweet.PermanentURL,
				tweet.Text,
				tweet.IsQuoted,
				tweet.IsPin,
				tweet.IsReply,
				tweet.IsRetweet,
				tweet.IsSelfThread,
				tweet.Views,
				tweet.Likes,
				tweet.Retweets,
				tweet.Replies,
			})
		}
		if spreadsheetID != "" && downloadedTweets%batchSize == 0 && downloadedTweets > 0 {
			tweetsToUpload := len(spreadsheetDataBuf)
			_, err := srv.Spreadsheets.Values.Append(spreadsheetID, "Sheet1!A1", &sheets.ValueRange{
				MajorDimension: "ROWS",
				Values:         spreadsheetDataBuf,
			}).ValueInputOption("RAW").Do()
			if err != nil {
				panic(fmt.Errorf("unable to append %d tweets to Google Sheet: %w", tweetsToUpload, err))
			}
			fmt.Printf("Uploaded %d tweets to Google Sheets (latest ID: %s)...\n", tweetsToUpload, tweet.ID)
			spreadsheetDataBuf = spreadsheetDataBuf[:0]
		}
		downloadedTweets++
	}
	tweetsToUpload := len(spreadsheetDataBuf)
	if spreadsheetID != "" && tweetsToUpload > 0 {
		_, err := srv.Spreadsheets.Values.Append(spreadsheetID, "Sheet1!A1", &sheets.ValueRange{
			MajorDimension: "ROWS",
			Values:         spreadsheetDataBuf,
		}).ValueInputOption("RAW").Do()
		if err != nil {
			panic(fmt.Errorf("unable to append %d tweets to Google Sheet: %w", tweetsToUpload, err))
		}
		fmt.Printf("Uploaded last %d tweets to Google Sheets...\n", tweetsToUpload)
	}
	fmt.Printf("Downloaded %d tweets\n", downloadedTweets)
	_, err = f.WriteString("] }")
	if err != nil {
		panic(fmt.Errorf("unable to finish writing to output file: %w", err))
	}
}
