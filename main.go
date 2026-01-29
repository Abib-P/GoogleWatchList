package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache OAuth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

func retrieveSheetIdFromEnvironment() string {
	sheetID := os.Getenv("GOOGLE_SHEET_ID")
	if sheetID == "" {
		log.Fatal("GOOGLE_DOC_ID environment variable is not set")
	}
	return sheetID
}

func exitAppIfDuplicatedIsDetected(resp *sheets.ValueRange, uniqueRows map[string][]interface{}) {
	for _, row := range resp.Values {
		if len(row) > 0 {
			rowKey := ""
			for _, col := range row {
				if rowKey != "" {
					rowKey += "|"
				}
				rowKey += fmt.Sprintf("%v", col)
			}
			if _, exists := uniqueRows[rowKey]; !exists {
				uniqueRows[rowKey] = row
			} else {
				fmt.Printf("Duplicate row found: %v\n", row)
				os.Exit(1)
			}
		}
	}
}

func retrieveTmdbApiKeyFromEnvironment() string {
	tmdbApiKey := os.Getenv("TMDB_API_KEY")
	if tmdbApiKey == "" {
		log.Fatal("TMDB_API_KEY environment variable is not set")
	}
	return tmdbApiKey
}

func searchTmdbMovie(tmdbApiKey string, query string, year []string, language []string) {
	url := fmt.Sprintf("https://api.themoviedb.org/3/search/movie?query=%s", query)
	if len(language) > 0 {
		url += fmt.Sprintf("&language=%s", language[0])
	}
	if len(year) > 0 {
		url += fmt.Sprintf("&year=%s", year[0])
	}

	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+tmdbApiKey)

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	//if only 1 result, print it
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Error unmarshalling TMDB response: %v", err)
	}
	if results, ok := result["results"].([]interface{}); ok {
		if len(results) == 1 {
			if movie, ok := results[0].(map[string]interface{}); ok {
				fmt.Printf("Found TMDB ID %v for title %s\n", movie["id"], query)
				return
			}
		} else if len(results) > 1 {
			// trouver une methode pour choisir le bon film parmi les multiples résultats
		} else {
			fmt.Printf("No results found for title %s\n", query)
		}
	} else {
		log.Fatalf("Unexpected TMDB response format")
	}
}

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/spreadsheets.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	sheetID := retrieveSheetIdFromEnvironment()
	sheetRange := "Movies!A2:Z"
	resp, err := srv.Spreadsheets.Values.Get(sheetID, sheetRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	if len(resp.Values) == 0 {
		log.Println("No data found.")
		return
	}

	uniqueRows := make(map[string][]interface{})
	exitAppIfDuplicatedIsDetected(resp, uniqueRows)

	//now need to do search in tmdb to find tmdb id for each movie
	tmdbApiKey := retrieveTmdbApiKeyFromEnvironment()
	for _, column := range resp.Values {
		if len(column) == 0 {
			continue
		}
		titleStr := fmt.Sprintf("%v", column[0])
		title := strings.TrimSpace(titleStr)
		println("Searching TMDB for title: " + title + " | year " + fmt.Sprintf("%v", column[2]))
		searchTmdbMovie(tmdbApiKey, title, []string{fmt.Sprintf("%v", column[2])}, []string{"en-US"})
	}
}
