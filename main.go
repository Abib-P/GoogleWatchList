package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"os"
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

func retrieveDocumentIdFromEnvironment() string {
	docId := os.Getenv("GOOGLE_DOC_ID")
	if docId == "" {
		log.Fatal("GOOGLE_DOC_ID environment variable is not set")
	}
	return docId
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
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/documents.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := docs.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Docs client: %v", err)
	}

	docId := retrieveDocumentIdFromEnvironment()
	doc, err := srv.Documents.Get(docId).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from document: %v", err)
	}

	var text string
	for _, element := range doc.Body.Content {
		if element.Paragraph != nil {
			for _, paraElement := range element.Paragraph.Elements {
				if paraElement.TextRun != nil {
					text += paraElement.TextRun.Content
				}
			}
		}
	}

	lines := splitLines(text)
	for i, line := range lines {
		if len(line) > 0 && line[0] >= 'a' && line[0] <= 'z' {
			lines[i] = string(line[0]-32) + line[1:] // Convert first character to uppercase
		}
	}

	for i := 0; i < len(lines)-1; i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[i] > lines[j] {
				lines[i], lines[j] = lines[j], lines[i]
			}
		}
	}

	uniqueLines := make([]string, 0)
	for i, line := range lines {
		if i == 0 || line != lines[i-1] {
			uniqueLines = append(uniqueLines, line)
		}
	}

	for _, line := range uniqueLines {
		fmt.Println(line)
	}
}
