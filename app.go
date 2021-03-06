package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/magiconair/properties"
	"github.com/newrelic/go-agent"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type App struct {
	Router *mux.Router
	DB     WeddingDatabase
}

func (a *App) Initialize(dbUser string, dbPassword string, newRelicAppName string, newRelicLicenseKey string) {
	dbHost := "127.0.0.1"
	dbPort := 3306

	db, err := NewMySQLDB(MySQLConfig{Username: dbUser, Password: dbPassword, Host: dbHost, Port: dbPort})
	if err != nil {
		log.Fatal("Unable to connect to database: ", err)
		os.Exit(1)
	}

	config := newrelic.NewConfig(newRelicAppName, newRelicLicenseKey)
	app, err2 := newrelic.NewApplication(config)

	if err2 != nil {
		log.Fatal("Unable to create new relic application: ", err2)
		os.Exit(1)
	}

	a.DB = db
	a.Router = mux.NewRouter()
	a.initializeRoutes(app)
}

func (a *App) Run(addr string) {
	log.Printf("Starting... on %s", addr)
	log.Fatal(http.ListenAndServe(addr, a.Router))
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (a *App) getProperties() map[string]string {
	var p map[string]string
	p = properties.MustLoadFile("static/uk.properties", properties.UTF8).Map()
	return p
}

func (a *App) handler(w http.ResponseWriter, r *http.Request) {
	m := a.getProperties()
	if r.URL.Path == "/" {
		t, _ := template.ParseFiles("templates/index.tmpl")
		err := t.Execute(w, m)
		if err != nil {
			log.Print("Unable to parse template: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if r.URL.Path == "/ping" {
		fmt.Fprintf(w, "OK")
	} else {
		t, _ := template.ParseFiles(fmt.Sprintf("templates/%s.tmpl", r.URL.Path))
		err := t.Execute(w, m)
		if err != nil {
			log.Print("Unable to parse template: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type Page struct {
	Rsvp *Rsvp
	P    map[string]string
}

func (a *App) ShowRsvp(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	log.Printf("ShowRsvp(%s)", params["id"])

	item, err := a.DB.GetRsvp(params["id"])

	if err != nil {
		log.Print("Invalid reference: ", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	t, _ := template.ParseFiles(fmt.Sprintf("templates/%s.tmpl", "show_rsvp"))
	err2 := t.Execute(w, Page{Rsvp: item, P: a.getProperties()})
	if err2 != nil {
		log.Print("Unable to parse template: ", err2)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) ShowInvite(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	log.Printf("ShowInvite(%s)", params["id"])

	item, err := a.DB.GetRsvp(params["id"])

	if err != nil {
		log.Print("Invalid reference: ", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	t, _ := template.ParseFiles(fmt.Sprintf("templates/%s.tmpl", "show_invite"))
	err2 := t.Execute(w, Page{Rsvp: item, P: a.getProperties()})
	if err2 != nil {
		log.Print("Unable to parse template: ", err2)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) ShowRsvpRest(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	log.Printf("ShowRsvp(%s)", params["id"])

	item, err := a.DB.GetRsvp(params["id"])

	if err != nil {
		log.Print("Invalid reference: ", err)
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Unable to find '%s'", params["id"]))
		return
	}

	respondWithJSON(w, http.StatusOK, item)
}

func (a *App) SaveRsvp(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	log.Printf("SaveRsvp(%s)", params["id"])

	item, err := a.DB.GetRsvp(params["id"])
	log.Printf("Got Rsvp from DB\n%s", item)

	if err != nil {
		log.Print("Invalid reference: ", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if r.ParseForm() != nil {
		log.Print("Unable to parse form")
	}

	log.Printf("Form: %s", r.PostForm)
	decoder := schema.NewDecoder()
	err2 := decoder.Decode(item, r.PostForm)

	log.Printf("Got Rsvp after setting values from form\n%s", item)

	if err2 != nil {
		log.Print("Unable to decode rsvp:", err2)
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}

	item.ReplyType = "web"
	if item.IsAttending() {
		item.ReplyStatus = "attending"
	} else {
		item.ReplyStatus = "notattending"
	}

	a.DB.UpdateRsvp(item)

	if item.IsAttending() {
		target := "http://" + r.Host + "/attending"
		log.Print("Sending Redirect: " + target)
		http.Redirect(w, r, target, http.StatusSeeOther)
	}

	target := "http://" + r.Host + "/notattending"
	log.Print("Sending Redirect: " + target)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (a *App) SaveRsvpRest(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id := params["id"]
	log.Printf("SaveRsvpRest(%s)", id)

	item, err := a.DB.GetRsvp(id)
	log.Printf("Got Rsvp from DB\n%s", item)

	if err != nil {
		log.Print("Invalid reference: ", err)
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("Invalid reference %s", id))
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Print("Unable to decode rsvp:", err)
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("unable to decode rsvp: %s", err))
		return
	}

	err = json.Unmarshal(body, item)
	log.Printf("Got Rsvp after setting values from request\n%s", item)

	if err != nil {
		log.Print("Unable to decode rsvp:", err)
		respondWithError(w, http.StatusNotFound, fmt.Sprintf("unable to decode rsvp: %s", err))
		return
	}

	item.ReplyType = "api"
	if item.ReplyStatus == "attending" {
		for _, g := range item.Guests {
			g.Attending = true
		}
	} else if item.ReplyStatus == "notattending" {
		for _, g := range item.Guests {
			g.Attending = false
		}
	} else {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid status %s", item.ReplyStatus))
	}

	a.DB.UpdateRsvp(item)

	target := "http://" + r.Host + "/api/rsvp/" + item.RsvpID
	log.Print("Sending Redirect: " + target)
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (a *App) initializeRoutes(nr newrelic.Application) {
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/", a.handler))
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/ping", a.handler))
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/api", a.handler))
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/attending", a.handler))
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/notattending", a.handler))

	// web calls
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/invite/{id}", a.ShowInvite)).Methods("GET")
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/rsvp/{id}", a.ShowRsvp)).Methods("GET")
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/rsvp/{id}/save", a.SaveRsvp)).Methods("POST")

	// api calls
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/api/rsvp/{id}", a.ShowRsvpRest)).Methods("GET")
	a.Router.HandleFunc(newrelic.WrapHandleFunc(nr, "/api/rsvp/{id}", a.SaveRsvpRest)).Methods("POST")

	// static calls
	a.Router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
}
