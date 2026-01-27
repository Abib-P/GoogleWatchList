package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	"log"
	"net/http"
	"os"
	"strings"
)

// Retrieves a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Requests a token from the web, then returns the retrieved token.
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

// Retrieves a token from a local file.
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

// Saves a token to a file path.
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

func splitLines(text string) []string {
	lines := make([]string, 0)
	currentLine := ""
	for _, char := range text {
		if char == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(char)
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
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

	uppercaseFirstColumn := make([]string, 0)
	for i, row := range resp.Values {
		if len(row) > 0 {
			if firstCol, ok := row[0].(string); ok {
				words := strings.Fields(firstCol)
				for i, word := range words {
					if len(word) > 0 {
						// Todo : fix '-' and other special characters
						words[i] = strings.ToUpper(string(word[0])) + word[1:]
					}
				}
				// Join the words back together
				uppercaseFirstColumn = append(uppercaseFirstColumn, strings.Join(words, " "))
				resp.Values[i][0] = uppercaseFirstColumn[i]
			}
		}
	}

	uniqueRows := make(map[string][]interface{})
	exitAppIfDuplicatedIsDetected(resp, uniqueRows)

	//now need to do search in tmdb to find tmdb id for each movie

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
