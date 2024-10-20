package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type SearchResult struct {
	Images   []string `json:"images"`
	Bookmark string   `json:"bookmark,omitempty"`
}

var allowedDomains = []string{"pinimg.com", "i.pinimg.com", "pinterest.com"}

func main() {
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	router := gin.Default()

	router.Static("/static", "./static")
	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	router.GET("/search/pins/", searchHandler)
	router.GET("/image", proxyImageHandler)
	router.GET("/donate", func(c *gin.Context) {
		c.HTML(http.StatusOK, "donate.html", nil)
	})
	router.GET("/licenses", func(c *gin.Context) {
		c.HTML(http.StatusOK, "licenses.html", nil)
	})

	fmt.Println("Server started")
	router.Run(":3000")
}

func searchHandler(c *gin.Context) {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	URL := os.Getenv("URL")
	if URL == "" {
		fmt.Println("URL not set in .env file")
		return
	}
	query := c.Query("q")
	bookmark := c.Query("bookmark")
	csrftoken := c.Query("csrftoken")

	apiURL := "https://www.pinterest.com/resource/BaseSearchResource/get/"
	dataParamObj := map[string]interface{}{
		"options": map[string]interface{}{
			"query":     query,
			"bookmarks": []string{bookmark},
		},
	}

	dataParam, err := json.Marshal(dataParamObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode data"})
		return
	}

	finalURL := fmt.Sprintf("%s?data=%s", apiURL, url.QueryEscape(string(dataParam)))

	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	if csrftoken != "" {
		req.Header.Set("x-csrftoken", csrftoken)
		req.Header.Set("cookie", fmt.Sprintf("csrftoken=%s", csrftoken))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Request failed"})
		return
	}
	defer resp.Body.Close()

	var responseData struct {
		ResourceResponse struct {
			Data struct {
				Results []struct {
					Images struct {
						Orig struct {
							URL string `json:"url"`
						} `json:"orig"`
					} `json:"images"`
				} `json:"results"`
			} `json:"data"`
			Bookmark string `json:"bookmark,omitempty"`
		} `json:"resource_response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response"})
		return
	}

	var images []string
	for _, result := range responseData.ResourceResponse.Data.Results {
		imageUrl := result.Images.Orig.URL
		if imageUrl != "" && isAllowedDomain(imageUrl) {
			proxyImageUrl := fmt.Sprintf("%s/image?url=%s", URL, url.QueryEscape(imageUrl))
			images = append(images, proxyImageUrl)
		}
	}

	c.HTML(http.StatusOK, "results.html", gin.H{
		"Images":    images,
		"Bookmark":  responseData.ResourceResponse.Bookmark,
		"Query":     query,
		"CSRFToken": csrftoken,
	})
}

func proxyImageHandler(c *gin.Context) {
	imageUrl := c.Query("url")
	if !isAllowedDomain(imageUrl) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Domain not allowed"})
		return
	}

	imageSrc, err := fetchImage(imageUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch image"})
		return
	}

	c.Header("Content-Type", "image/png")
	c.Data(http.StatusOK, "image/png", imageSrc)
}

func isAllowedDomain(urlStr string) bool {
	parsedUrl, err := url.Parse(urlStr)
	if err != nil || parsedUrl.Host == "" {
		return false
	}

	for _, domain := range allowedDomains {
		if parsedUrl.Host == domain || strings.HasSuffix(parsedUrl.Host, "."+domain) {
			return true
		}
	}

	return false
}

func fetchImage(imageUrl string) ([]byte, error) {
	resp, err := http.Get(imageUrl)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch image")
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
