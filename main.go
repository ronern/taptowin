package main

import (
	"context"
	"encoding/json"
	"fmt"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jackc/pgx/v4"
	"log"
	"net/http"
	"os"
)

var conn *pgx.Conn

type Info struct {
	Energy int32
	Money  float32

	EnergyTimer      uint64
	EnergyVideoTimer uint64

	Bet1    int32
	Bet10   int32
	Bet100  int32
	Bet1000 int32
}

func getArg(req *http.Request, name string) string {
	args, ok := req.URL.Query()[name]

	if !ok || len(args[0]) < 1 {
		return ""
	}

	return args[0]
}

func getInfoHandler(w http.ResponseWriter, req *http.Request) {

	ids, ok := req.URL.Query()["id"]

	if !ok || len(ids[0]) < 1 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	id := ids[0]

	var info Info

	row := conn.QueryRow(context.Background(), "select energy, money, bet1, bet10, bet100, bet1000, energyTimer, energyVideoTimer from users where id=$1", id)
	err := row.Scan(&info.Energy, &info.Money, &info.Bet1, &info.Bet10, &info.Bet100, &info.Bet1000, &info.EnergyTimer, &info.EnergyVideoTimer)

	if err == pgx.ErrNoRows {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	infoJson, err := json.Marshal(info)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, infoJson)
}

func registerHandler(w http.ResponseWriter, req *http.Request) {

	var id string
	var name string

	id = getArg(req, "id")
	name = getArg(req, "name")

	if len(id) == 0 || len(name) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	_, err := conn.Exec(context.Background(), "INSERT INTO users (id, name) VALUES ('$1', '$2') ON CONFLICT DO NOTHING", id, name)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "OK")
}

func headers(w http.ResponseWriter, req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
}

func main() {

	var err error
	conn, err = pgx.Connect(context.Background(), "postgres://tnqhqsdfdamjoa:47e01ba2a9d708fa5dcdf10653b64e7c588ca9eef2befdf3415a8eaee4d318d2@ec2-54-75-225-52.eu-west-1.compute.amazonaws.com:5432/d516nrb1815o1")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	//defer conn.Close(context.Background())

	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	http.HandleFunc("/info", getInfoHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/headers", headers)
	http.ListenAndServe(":"+port, nil)
}
