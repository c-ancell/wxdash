package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

// LookupData is a struct type that represents the info sent to the frontend when a lookup is performed
type LookupData struct {
	ZipCode     string
	City        string
	State       string
	Temperature float64
	Station     string
}

// LocationData is a struct type that represents a location in the USA
type LocationData struct {
	City      string
	State     string
	Zip       string
	Latitude  float64
	Longitude float64
}

// WeatherObservation is a struct type that represents an observation from one of NOAA's observation stations
type WeatherObservation struct {
	Station     string
	Temperature float64
}

func homeHandler(w http.ResponseWriter, req *http.Request) {

	// Using ParseFiles here is what lets us store templates in their own files.
	// Remember template names should have same name as files to work smoothly.
	templ, err := template.New("home").ParseFiles("home")
	if err != nil {
		fmt.Fprintf(w, "There was an error generating the template for this page!")
		panic(err)
	}

	err = templ.Execute(w, nil)
}

func lookupHandler(w http.ResponseWriter, req *http.Request) {
	// Have to run this to init the form values
	req.ParseForm()

	// We'll load a new page here, so need a new template
	templ, err := template.New("lookup").ParseFiles("lookup")
	if err != nil {
		fmt.Fprintf(w, "There was an error generating the template for this page!")
		panic(err)
	}

	// Parse the user entered zip code and turn it into an int
	zip := req.Form.Get("zipCode")
	lookup := getObservationForZip(zip)

	// Notice the data type here - used to send info into the templates. Might need to be re-worked
	err = templ.Execute(w, lookup)

	// TODO: Use zip code to get current temperature
}

// latLonForZip takes a given zip code as a string and uses the OpenDataSoft API to gather location info relative to that zip code.
// It returns a locationData struct, which packages the relative info in an easy to use data type.
func latLonForZip(zip string) LocationData {

	// Build the API request URL using the given zip code.
	url := fmt.Sprintf("https://public.opendatasoft.com/api/records/1.0/search/?dataset=us-zip-code-latitude-and-longitude&q=%v", zip)

	// This is where we will unpack the JSON data response into a locationData struct. First, we build our struct and the container
	// to hold the response body (jsonData)
	var loc LocationData
	jsonData := getJSONData(url)

	// This is where it gets fun. The location data we need is buried inside the JSON from the API. It's nice to have so much info, but
	// much of it is unneeded for our purposes.
	// TODO: Find out if there's a better, cleaner way of doing this instead of drilling down with all of these type assertions. Tags?
	typedJSONData := jsonData.(map[string]interface{})

	for k := range typedJSONData {
		if k == "records" {
			typedRecordsInfo := typedJSONData["records"].([]interface{})
			typedInfoRecord := typedRecordsInfo[0].(map[string]interface{})
			for k := range typedInfoRecord {
				if k == "fields" {
					typedLocData := typedInfoRecord["fields"].(map[string]interface{})
					for k = range typedLocData {
						switch k {
						case "city":
							loc.City = typedLocData[k].(string)
						case "zip":
							loc.Zip = typedLocData[k].(string)
						case "longitude":
							loc.Longitude = typedLocData[k].(float64)
						case "state":
							loc.State = typedLocData[k].(string)
						case "latitude":
							loc.Latitude = typedLocData[k].(float64)
						}
					}
				}
			}
		}

	}

	return loc
}

// getJSONData makes a GET request to the specified URL and returns what is essentially the body of the response
func getJSONData(requestURL string) interface{} {
	resp, err := http.Get(requestURL)
	if err != nil {
		fmt.Fprintf(os.Stdout, "There was an error making the request to the location info API: %v\n", err)

		return nil
	}
	defer resp.Body.Close()

	var jsonData interface{}

	err = json.NewDecoder(resp.Body).Decode(&jsonData)
	if err != nil {
		fmt.Fprintf(os.Stdout, "There was an error processing JSON data: %v\n", err)
		return nil
	}

	return jsonData
}

func findNearestStation(lat, lon float64) string {

	var stationID string

	// First, build the request URL
	url := fmt.Sprintf("https://api.weather.gov/points/%v,%v/stations", lat, lon)
	jsonData := getJSONData(url)
	typedJSONData := jsonData.(map[string]interface{})
	for k := range typedJSONData {
		if k == "observationStations" {
			typedStations := typedJSONData["observationStations"].([]interface{})
			for k := range typedStations {
				if k == 0 {
					stationID = typedStations[k].(string)
					stationID = stationID[len(stationID)-4:]
					return stationID
				}
			}
		}
	}

	return stationID
}

func getObservationForZip(zip string) LookupData {

	// Get location info for the requested ZIP code.
	loc := latLonForZip(zip)

	// Look up nearest station from weather.gov API for that lat/lon pair
	nearestStation := findNearestStation(loc.Latitude, loc.Longitude)

	// Get latest observation from station, get temperature
	latestObservation := getLatestObservation(nearestStation)
	return LookupData{
		City:        loc.City,
		State:       loc.State,
		ZipCode:     loc.Zip,
		Temperature: latestObservation.Temperature,
		Station:     latestObservation.Station,
	}

}

func getLatestObservation(station string) WeatherObservation {

	latestObservation := WeatherObservation{station, 0}

	requestURL := fmt.Sprintf("https://api.weather.gov/stations/%v/observations/latest", station)
	jsonData := getJSONData(requestURL)
	typedJSONData := jsonData.(map[string]interface{})
	for k, v := range typedJSONData {
		if k == "properties" {
			typedProps := v.(map[string]interface{})
			for k, v := range typedProps {
				if k == "temperature" {
					typedTemp := v.(map[string]interface{})
					for k, v := range typedTemp {
						if k == "value" {
							latestObservation.Temperature = v.(float64)
						}
					}
				}
			}
		}
	}

	return latestObservation
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/lookup", lookupHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
