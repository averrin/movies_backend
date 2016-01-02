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
	// "log"
	"net/http"
	"os"
	"fmt"
	"regexp"
	"bytes"
	log "github.com/Sirupsen/logrus"
	// "github.com/jmcvetta/randutil"
	// "io/ioutil"
)

type Response struct {
	Text string `json:"text"`
}

type MovieForm struct {
	RawIMDB  string `json:"imdb"`
}

type User struct {
	ClientID       string `json:"clientID"`
	CreatedAt      string `json:"created_at"`
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	FamilyName     string `json:"family_name"`
	Gender         string `json:"gender"`
	GivenName      string `json:"given_name"`
	GlobalClientID string `json:"global_client_id"`
	Identities     []struct {
		AccessToken string `json:"access_token"`
		Connection  string `json:"connection"`
		ExpiresIn   int    `json:"expires_in"`
		IsSocial    bool   `json:"isSocial"`
		Provider    string `json:"provider"`
		UserID      string `json:"user_id"`
	} `json:"identities"`
	Locale       string `json:"locale"`
	Name         string `json:"name"`
	Nickname     string `json:"nickname"`
	Picture      string `json:"picture"`
	UpdatedAt    string `json:"updated_at"`
	UserID       string `json:"user_id"`
	UserMetadata struct {
		Admin     bool `json:"admin"`
		Superuser bool `json:"superuser"`
	} `json:"user_metadata"`
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
	AuthorID   string `json:"authorID"`
	Author  	 User   `json:"author"`
	Seen			 bool   `json:"seen"`
	Rate			 int    `json:"rate"`
	Rates      []Rate `json:"rates"`
	Index			 float32		`json:"index"`
}

type Rate struct {
	AuthorID   		string `json:"authorID"`
	ImdbID     		string `json:"imdbID"`
	Seen			 		bool   `json:"seen"`
	Rate			 		int    `json:"rate"`
	AuthorName 		string `json:"authorName"`
	AuthorAvatar	string `json:"authorAvatar"`
}

type key int

const db key = 0

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return
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

func AddUser(token *jwt.Token, db *mgo.Database) {

	url := "https://averrin.auth0.com/tokeninfo"

	// t, _ := json.Marshal(token)
	fmt.Println(token.Raw)
	var jsonStr = []byte(`{"id_token":"`+token.Raw+`"}`)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
			panic(err)
	}
	var user User;
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&user)
	// u, _ := json.Marshal(user)
	// fmt.Println(string(u))
	defer resp.Body.Close()
	db.C(`users`).Insert(&user)
}

func MongoMiddleware() negroni.HandlerFunc {
	session, err := mgo.Dial("mongo")
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
	log.Info("Setting routes")
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
		negroni.Wrap(http.HandlerFunc(restMovies)),
	)).Methods("POST", "GET")
	r.Handle("/movies/{imdbID}", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.HandlerFunc(MongoMiddleware()),
		negroni.Wrap(http.HandlerFunc(restMovie)),
	)).Methods("DELETE", "POST")
	http.Handle("/", r)
	port := os.Getenv("PORT")
	log.Info("Start listening port " + port)
	http.ListenAndServe(":" + port, nil)
}

func Find(vs []User, f func(User) bool) User {
    for _, v := range vs {
        if f(v) {
						return v
        }
    }
    return User{}
}

func FindRate(vs []Rate, f func(Rate) bool) Rate {
    for _, v := range vs {
        if f(v) {
						return v
        }
    }
    return Rate{Seen: false, Rate: 0}
}

func restMovie(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
  imdbID := vars["imdbID"]
	user := context.Get(req, "user")
	uid := user.(*jwt.Token).Claims[`sub`]
	var movie Movie;
	db := GetDb(req)
	c := db.C(`movies`)
	r := db.C(`rates`)
	u := db.C(`users`)
	err := c.Find(bson.M{"imdbid": imdbID}).One(&movie)
	if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusNotFound)
			return
	}
	switch req.Method {
		case "DELETE":
			if uid.(string) != movie.AuthorID {
					var usr User;
					u.Find(bson.M{`userid`: uid.(string)}).One(&usr)
					if usr.UserMetadata.Admin != true {
						log.Println(`wrong author`)
						http.Error(w, `wrong author`, http.StatusForbidden)
						return
					}
			}
			c.Remove(bson.M{"imdbid": imdbID})
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`Deleted`))
		case "POST":
			q := db.C(`users`).Find(bson.M{"userid": uid}).Limit(1)
			count, _ := q.Count()
			if count == 0 {
				go AddUser(user.(*jwt.Token), db)
			}
			rate := new(Rate)
			decoder := json.NewDecoder(req.Body)
			decoder.Decode(&rate)
			rate.AuthorID = uid.(string)
			rate.ImdbID = imdbID
			fmt.Println(rate)
			_, err := r.Upsert(bson.M{"imdbid": imdbID, "authorid": uid.(string)}, bson.M{"$set": rate})
			if err != nil {
					log.Println(err.Error())
					http.Error(w, err.Error(), http.StatusNotFound)
					return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`Updated`))
	}
}

func restMovies(w http.ResponseWriter, req *http.Request) {
	user := context.Get(req, "user")
	uid := user.(*jwt.Token).Claims[`sub`]
	db := GetDb(req)
	c := db.C(`movies`)
	r := db.C(`rates`)
	u := db.C(`users`)
	switch req.Method {
		case "GET":
			var movies []Movie;
			var rates []Rate;
			var all_rates []Rate;
			var users []User;
			err := c.Find(nil).All(&movies)
			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			r.Find(bson.M{"authorid": uid.(string)}).All(&rates)
			r.Find(nil).All(&all_rates)
			u.Find(nil).All(&users)
			result := make([]Movie, 0)
			for _, movie := range movies {
				// fmt.Println(i, movie)
				movie.Author = Find(users, func(u User) bool {
					return u.UserID == movie.AuthorID
				})
				rate := FindRate(rates, func(r Rate) bool {
					return r.ImdbID == movie.ImdbID
				})
				movie.Seen = rate.Seen;
				movie.Rate = rate.Rate;
				index := float32(0);
				count := float32(len(users));
				for _, r := range all_rates {
					if r.ImdbID == movie.ImdbID {
						index += float32(r.Rate)
						// count += float32(1)
						if r.AuthorID != uid.(string) {
							author := Find(users, func(u User) bool {
								return u.UserID == r.AuthorID
							})
							r.AuthorName = author.Name
							r.AuthorAvatar = author.Picture
							movie.Rates = append(movie.Rates, r)
						}
					}
				}
				if count > 0 {
					movie.Index = index / count
				} else {
					movie.Index = float32(0)
				}
				result = append(result, movie)
			}
			w.Header().Set("Content-Type", "application/json")
			r, _ := json.Marshal(result)
			// r, _ := json.Marshal(movies)
			w.Write(r)
		case "POST":
			idPattern := regexp.MustCompile(`tt\d+`)
			q := db.C(`users`).Find(bson.M{"userid": uid}).Limit(1)
			count, err := q.Count()
			if count == 0 {
				go AddUser(user.(*jwt.Token), db)
			}

			form := new(MovieForm)
			decoder := json.NewDecoder(req.Body)
			error := decoder.Decode(&form)
			if error != nil {
				log.Println(error.Error())
				http.Error(w, error.Error(), http.StatusInternalServerError)
				return
			}
			id := idPattern.FindString(form.RawIMDB)
			url := `http://www.omdbapi.com/?i=` + id + `&plot=full&r=json`
			resp, err := http.Get(url)
			if err != nil {
				log.Println(err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer resp.Body.Close()

			movie := new(Movie)
			author := User{}
			q.One(&author)
			movie.Author = author
			movie.AuthorID = uid.(string)
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
			count, err = c.Find(bson.M{"imdbid": movie.ImdbID}).Limit(1).Count()
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
