package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

type Product struct {
	Url   string `json:"url"`
	Image string `json:"image"`
	Name  string `json:"name"`
	Price string `json:"price"`
}

type AllProducts struct {
	Prd []Product `json:"product"`
}

var visitedPages []string
var visitedMutex = &sync.Mutex{}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		products := getProducts("https://www.scrapingcourse.com/ecommerce/")
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(AllProducts{Prd: products})
		if err != nil {
			io.WriteString(w, fmt.Sprintf("{ \"error\": \"%s\"}", err.Error()))
			return
		}
	})

	err := http.ListenAndServe(":3333", nil)
	if errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("server closed\n")
	} else if err != nil {
		fmt.Printf("error starting server: %s\n", err)
		os.Exit(1)
	}
}

func getProducts(url string) []Product {
	allProducts := make(chan []Product)
	go parsePage(allProducts, url, 0)

	return <-allProducts
}

func parsePage(list chan []Product, page string, depth int) {
	visitedMutex.Lock()
	visitedPages = append(visitedPages, page)
	visitedMutex.Unlock()

	fmt.Printf("Parsing page %s (%d)\n", page, depth)
	res, err := http.Get(page)
	if err != nil {
		fmt.Printf("Error making request %s\n", err)
		os.Exit(1)
	}
	defer res.Body.Close()
	var retProducts []Product

	allPages := []string{}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	doc.Find("#pagination a").Each(func(i int, s *goquery.Selection) {
		page, exists := s.Attr("href")
		if exists && !contains(allPages, page) {
			allPages = append(allPages, page)
		}
	})

	doc.Find("li.product").Each(func(i int, s *goquery.Selection) {
		p := s.Find("a").First()
		url, existsHref := p.Attr("href")
		name := p.Find("h2").First().Text()
		image, existsImage := p.Find("img").Attr("src")
		price := p.Find("bdi").Text()
		if existsHref && existsImage {
			retProducts = append(retProducts, Product{Name: name, Url: url, Image: image, Price: price})
		}
	})
	if depth < 5 { // Limit how deep we can go (should be less than this)
		// Recurse to new pages
		var unVisitedPages []string
		for _, p := range allPages {
			var visited bool
			visitedMutex.Lock()
			visited = contains(visitedPages, p)
			visitedMutex.Unlock()
			if !visited {
				unVisitedPages = append(unVisitedPages, p)
			}
		}
		if len(unVisitedPages) > 0 {
			allProducts := make(chan []Product, len(unVisitedPages))
			for _, p := range unVisitedPages {
				go parsePage(allProducts, p, depth+1)
			}

			for range unVisitedPages {
				pageProducts := <-allProducts
				retProducts = appendUnique(retProducts, pageProducts)
			}
			close(allProducts)
		}
	}
	list <- retProducts
}

func mapSlice[T any, M any](a []T, f func(T) M) []M {
	n := make([]M, len(a))
	for i, e := range a {
		n[i] = f(e)
	}
	return n
}

func appendUnique(original []Product, newProducts []Product) []Product {
	ret := make([]Product, len(original))
	copy(ret, original)

	for _, item := range newProducts {
		if !contains(mapSlice(original, func(p Product) string { return p.Name }), item.Name) {
			ret = append(ret, item)
		}
	}

	return ret
}

func contains(slice []string, element string) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}
