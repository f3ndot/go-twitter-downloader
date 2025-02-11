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
var tweetsSheetID int64 = -1
var infoSheetID int64 = -1
var srv *sheets.Service
var tz *time.Location

const batchSize = 50

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <username>\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
}

func setupGoogleSheetsService(ctx context.Context) error {
	if spreadsheetID == "" {
		return nil
	}
	credentialsFile := "service_account.json"
	data, err := ioutil.ReadFile(credentialsFile)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}
	config, err := google.CredentialsFromJSON(ctx, data, sheets.SpreadsheetsScope)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	// Create Sheets service
	srv, err = sheets.NewService(ctx, option.WithCredentials(config))
	if err != nil {
		return fmt.Errorf("failed to create Sheets service: %w", err)
	}
	return nil
}

func prepareSpreadsheet(username string) error {
	if srv == nil {
		return fmt.Errorf("service not initialized")
	}

	fmt.Printf("Getting info for spreadsheet %s...\n", spreadsheetID)
	spreadsheet, err := srv.Spreadsheets.Get(spreadsheetID).Do()
	if err != nil {
		log.Fatalf("Failed to retrieve spreadsheet details: %v", err)
	}
	fmt.Printf("Resetting spreadsheet %s...\n", spreadsheetID)
	var requests []*sheets.Request
	for _, sheet := range spreadsheet.Sheets {
		sheetID := sheet.Properties.SheetId
		fmt.Println("Deleting Sheet:", sheet.Properties.Title, "Sheet ID:", sheetID)

		if sheet.Properties.Title == "Tweets" {
			tweetsSheetID = sheetID
			_, err := srv.Spreadsheets.Values.Clear(spreadsheetID, "Tweets", &sheets.ClearValuesRequest{}).Do()
			if err != nil {
				panic(fmt.Errorf("unable to clear Google Sheet: %w", err))
			}
		} else if sheet.Properties.Title == "Information" {
			infoSheetID = sheetID
			_, err := srv.Spreadsheets.Values.Clear(spreadsheetID, "Information", &sheets.ClearValuesRequest{}).Do()
			if err != nil {
				panic(fmt.Errorf("unable to clear Google Sheet: %w", err))
			}
		} else {
			requests = append(requests, &sheets.Request{
				DeleteSheet: &sheets.DeleteSheetRequest{
					SheetId: sheetID,
				},
			})
		}
	}
	if tweetsSheetID == -1 {
		requests = append(requests, &sheets.Request{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{
					Title: "Tweets",
				},
			},
		})
	}
	if infoSheetID == -1 {
		requests = append(requests, &sheets.Request{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{
					Title: "Information",
				},
			},
		})
	}

	if len(requests) > 0 {
		batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: requests,
		}
		results, err := srv.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdateRequest).Do()
		if err != nil {
			panic(fmt.Errorf("unable to reset Google Sheet: %w", err))
		}
		for _, reply := range results.Replies {
			if reply != nil && reply.AddSheet != nil && reply.AddSheet.Properties != nil {
				if reply.AddSheet.Properties.Title == "Tweets" {
					tweetsSheetID = reply.AddSheet.Properties.SheetId
				}
				if reply.AddSheet.Properties.Title == "Information" {
					infoSheetID = reply.AddSheet.Properties.SheetId
				}
			}
		}
	}

	h1, h2, h3, h4, h5, h6, h7, h8, h9, h10, h11, h12, h13 := "Date/Time (ET)",
		"Link",
		"Text",
		"IsQuoted",
		"IsPin",
		"IsReply",
		"IsRetweet",
		"IsSelfThread",
		"Views",
		"Likes",
		"Retweets",
		"Replies",
		"Quoted Tweet"
	fmt.Printf("Setting up spreadsheet %s...\n", spreadsheetID)
	requests = []*sheets.Request{
		{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: tweetsSheetID,
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
		{
			AppendCells: &sheets.AppendCellsRequest{
				Fields: "userEnteredValue",
				Rows: []*sheets.RowData{
					{
						Values: []*sheets.CellData{
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h1}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h2}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h3}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h4}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h5}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h6}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h7}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h8}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h9}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h10}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h11}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h12}},
							{UserEnteredValue: &sheets.ExtendedValue{StringValue: &h13}},
						},
					},
				},
				SheetId: tweetsSheetID,
			},
		},
	}
	// Send batch update request
	batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}
	_, err = srv.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdateRequest).Do()
	if err != nil {
		panic(fmt.Errorf("unable to prepare Google Sheet: %w", err))
	}
	return nil
}

func polishSpreadsheet() error {
	if srv == nil {
		return fmt.Errorf("service not initialized")
	}
	fmt.Printf("Polishing up spreadsheet %s...\n", spreadsheetID)
	requests := []*sheets.Request{
		{
			AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
				Dimensions: &sheets.DimensionRange{
					SheetId:    tweetsSheetID,
					Dimension:  "COLUMNS",
					StartIndex: 0, // Column A
					EndIndex:   1, // Up to (but not including) column B
				},
			},
		},
		{
			AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
				Dimensions: &sheets.DimensionRange{
					SheetId:    tweetsSheetID,
					Dimension:  "COLUMNS",
					StartIndex: 3,
					EndIndex:   12,
				},
			},
		},
		{
			UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
				Range: &sheets.DimensionRange{
					SheetId:    tweetsSheetID,
					Dimension:  "COLUMNS",
					StartIndex: 1, // Column B
					EndIndex:   2, // Only column B
				},
				Properties: &sheets.DimensionProperties{
					PixelSize: 90, // Fixed width of 200 pixels
				},
				Fields: "pixelSize",
			},
		},
		{
			UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
				Range: &sheets.DimensionRange{
					SheetId:    tweetsSheetID,
					Dimension:  "COLUMNS",
					StartIndex: 2,
					EndIndex:   3,
				},
				Properties: &sheets.DimensionProperties{
					PixelSize: 500,
				},
				Fields: "pixelSize",
			},
		},
		{
			UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
				Range: &sheets.DimensionRange{
					SheetId:    tweetsSheetID,
					Dimension:  "COLUMNS",
					StartIndex: 12,
					EndIndex:   13,
				},
				Properties: &sheets.DimensionProperties{
					PixelSize: 500,
				},
				Fields: "pixelSize",
			},
		},
		{
			UpdateDimensionProperties: &sheets.UpdateDimensionPropertiesRequest{
				Range: &sheets.DimensionRange{
					SheetId:    tweetsSheetID,
					Dimension:  "COLUMNS",
					StartIndex: 2,
					EndIndex:   3,
				},
				Properties: &sheets.DimensionProperties{
					PixelSize: 500,
				},
				Fields: "pixelSize",
			},
		},
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          tweetsSheetID,
					StartRowIndex:    0, // Entire column (all rows)
					StartColumnIndex: 12,
					EndColumnIndex:   13,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						WrapStrategy: "WRAP",
					},
				},
				Fields: "userEnteredFormat.wrapStrategy",
			},
		},
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          tweetsSheetID,
					StartRowIndex:    0, // Entire column (all rows)
					StartColumnIndex: 2,
					EndColumnIndex:   3,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						WrapStrategy: "WRAP",
					},
				},
				Fields: "userEnteredFormat.wrapStrategy",
			},
		},
	}
	// Send batch update request
	batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{Requests: requests}
	_, err := srv.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdateRequest).Do()
	if err != nil {
		panic(fmt.Errorf("unable to polish Google Sheet: %w", err))
	}

	_, err = srv.Spreadsheets.Values.Update(spreadsheetID, "Information!A1", &sheets.ValueRange{
		Values: [][]interface{}{
			{"Tweets Last Pulled (ET):", time.Now().In(tz).Format(time.DateTime)},
			{"Software:", "Justin's go-twitter-downloader"},
			{"Version:", "v0.1.0"},
		},
	}).ValueInputOption("USER_ENTERED").Do()
	if err != nil {
		panic(fmt.Errorf("unable to polish Google Sheet: %w", err))
	}

	return nil
}

func appendTweets(buf *[][]interface{}, latestTweetID string) error {
	if srv == nil {
		return fmt.Errorf("service not initialized")
	}
	tweetsToUpload := len(*buf)
	_, err := srv.Spreadsheets.Values.Append(spreadsheetID, "Tweets!A1", &sheets.ValueRange{
		MajorDimension: "ROWS",
		Values:         *buf,
	}).ValueInputOption("RAW").Do()
	if err != nil {
		return fmt.Errorf("unable to append %d tweets to Google Sheet: %w", tweetsToUpload, err)
	}
	fmt.Printf("Uploaded %d tweets to Google Sheets (latest ID: %s)...\n", tweetsToUpload, latestTweetID)
	*buf = (*buf)[:0]
	return nil
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

	var err error
	tz, err = time.LoadLocation("America/Toronto")
	if err != nil {
		panic(err)
	}

	scraper := twitterscraper.New()

	// Deserialize from JSON
	var authCookies AuthCookies
	cf, _ := os.Open("cookies.json")
	err = json.NewDecoder(cf).Decode(&authCookies)
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

	if spreadsheetID != "" {
		err = setupGoogleSheetsService(context.Background())
	}
	if err != nil {
		panic(err)
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
		err = prepareSpreadsheet(username)
	}
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
			quoteText := "N/A"
			if tweet.Tweet.QuotedStatus != nil {
				quoteText = fmt.Sprintf("From @%s: %s", tweet.Tweet.QuotedStatus.Username, tweet.Tweet.QuotedStatus.Text)
			}
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
				quoteText,
			})
		}
		if spreadsheetID != "" && downloadedTweets%batchSize == 0 && downloadedTweets > 0 {
			err := appendTweets(&spreadsheetDataBuf, tweet.ID)
			if err != nil {
				panic(err)
			}
		}
		downloadedTweets++
	}
	tweetsToUpload := len(spreadsheetDataBuf)
	if spreadsheetID != "" {
		if tweetsToUpload > 0 {
			err := appendTweets(&spreadsheetDataBuf, "")
			if err != nil {
				panic(err)
			}
			fmt.Printf("Uploaded last %d tweets to Google Sheets...\n", tweetsToUpload)
		}
		err = polishSpreadsheet()
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("Downloaded %d tweets\n", downloadedTweets)
	_, err = f.WriteString("] }")
	if err != nil {
		panic(fmt.Errorf("unable to finish writing to output file: %w", err))
	}
}
