package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/auth0/go-jwt-middleware"
	"github.com/codegangsta/negroni"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"gopkg.in/mgo.v2"
  "gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"os"
	"fmt"
	"regexp"
	// "io/ioutil"
)

type Response struct {
	Text string `json:"text"`
}

type MovieForm struct {
	RawIMDB  string `json:"imdb"`
}

type Movie struct {
	Actors     string `json:"Actors"`
	Awards     string `json:"Awards"`
	Country    string `json:"Country"`
	Director   string `json:"Director"`
	Genre      string `json:"Genre"`
	Language   string `json:"Language"`
	Metascore  string `json:"Metascore"`
	Plot       string `json:"Plot"`
	Poster     string `json:"Poster"`
	Rated      string `json:"Rated"`
	Released   string `json:"Released"`
	Response   string `json:"Response"`
	Runtime    string `json:"Runtime"`
	Title      string `json:"Title"`
	Type       string `json:"Type"`
	Writer     string `json:"Writer"`
	Year       string `json:"Year"`
	ImdbID     string `json:"imdbID"`
	ImdbRating string `json:"imdbRating"`
	ImdbVotes  string `json:"imdbVotes"`
	Author  	 string `json:"author,omitempty"`
}

type key int

const db key = 0

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	StartServer()

}

func GetDb(r *http.Request) *mgo.Database {
	if rv := context.Get(r, db); rv != nil {
		return rv.(*mgo.Database)
	}
	return nil
}

func SetDb(r *http.Request, val *mgo.Database) {
	context.Set(r, db, val)
}

func MongoMiddleware() negroni.HandlerFunc {
	session, err := mgo.Dial("localhost")
  if err != nil {
    panic(err)
  }

	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		reqSession := session.Clone()
		defer reqSession.Close()
		db := session.DB("movies")
		SetDb(r, db)
		next(rw, r)
	})
}

func StartServer() {
	r := mux.NewRouter().StrictSlash(true)

	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			decoded, err := base64.URLEncoding.DecodeString(os.Getenv("AUTH0_CLIENT_SECRET"))
			if err != nil {
				return nil, err
			}
			return decoded, nil
		},
	})

	r.Handle("/movies", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.HandlerFunc(MongoMiddleware()),
		negroni.Wrap(http.HandlerFunc(addMovie)),
	)).Methods("POST", "GET")
	http.Handle("/", r)
	http.ListenAndServe(":3001", nil)
}

func addMovie(w http.ResponseWriter, req *http.Request) {
	user := context.Get(req, "user")
	db := GetDb(req)
	c := db.C(`movies`)
	switch req.Method {
		case "GET":
			var movies []Movie;
			err := c.Find(nil).All(&movies)
			if err != nil {
					log.Println(err.Error())
					http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			w.Header().Set("Content-Type", "application/json")
			r, _ := json.Marshal(movies)
			w.Write(r)
		case "POST":
			idPattern := regexp.MustCompile(`tt\d+`)
			uid := user.(*jwt.Token).Claims[`sub`]
			form := new(MovieForm)
			decoder := json.NewDecoder(req.Body)
			error := decoder.Decode(&form)
			if error != nil {
					log.Println(error.Error())
					http.Error(w, error.Error(), http.StatusInternalServerError)
			}
			id := idPattern.FindString(form.RawIMDB)
			url := `http://www.omdbapi.com/?i=` + id + `&plot=full&r=json`
			resp, err := http.Get(url)
			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			defer resp.Body.Close()

			movie := new(Movie)
			movie.Author = uid.(string)
			MDecoder := json.NewDecoder(resp.Body)
			err = MDecoder.Decode(&movie)
			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if movie.Response == `False` {
				http.Error(w, `Wrong imdbID`, http.StatusInternalServerError)
				return
			}
			count, err := c.Find(bson.M{"imdbid": movie.ImdbID}).Limit(1).Count()
			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
		  }
		  if count > 0 {
				e := fmt.Sprintf("resource %s already exists", movie.ImdbID)
				log.Println(e)
				http.Error(w, e, http.StatusConflict)
				return
		  }
			err = c.Insert(&movie)
			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
		  }
			// respondJson(string(body), w)
			w.Header().Set("Content-Type", "application/json")
			r, _ := json.Marshal(movie)
			w.Write(r)
	}
}


func respondJson(text string, w http.ResponseWriter) {
	response := Response{text}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}
