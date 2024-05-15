// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//     - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	spotifyauth "github.com/zmb3/spotify/v2/auth"

	"github.com/zmb3/spotify/v2"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:8080/callback"

var (
	auth = spotifyauth.New(spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(spotifyauth.ScopeUserReadPrivate, spotifyauth.ScopeUserLibraryRead))
	ch    = make(chan *spotify.Client)
	state = "shuffler"
)

func main() {

	// first start an HTTP server
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	url := auth.AuthURL(state)
	fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	client := <-ch

	// use the client to make calls that require authorization
	user, err := client.CurrentUser(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("You are logged in as:", user.ID)

	playlists, err := client.GetPlaylistsForUser(context.Background(), user.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\nYour playlists:")
	fmt.Println("0 Saved Tracks")
	for i, pl := range playlists.Playlists {
		fmt.Println(i+1, pl.Name)
	}

	var toShuffle uint
	fmt.Println("\nEnter playlist number to shuffle: ")
	fmt.Scan(&toShuffle)

	for toShuffle > uint(playlists.Total) {
		fmt.Println("Error invalid playlist number")
		fmt.Println("\nEnter playlist number to shuffle: ")
		fmt.Scan(&toShuffle)
	}

	var allTracks []spotify.ID
	var playlistID spotify.ID

	if toShuffle == 0 {
		fmt.Printf("You have chosen playlist number 1 with title: Saved Tracks\n")
		allTracks = getSavedTracks(client)
		shuffle(allTracks)
		//TODO

	} else {
		fmt.Printf("You have chosen playlist number %d with title: %s\n", toShuffle, playlists.Playlists[toShuffle-1].Name)
		//fmt.Println(playlists.Playlists[toShuffle-1].Tracks)
		playlistID = playlists.Playlists[toShuffle-1].ID

		allTracks = getPlaylistTracks(client, playlistID)
		shuffle(allTracks)

		swapPlaylist(client, allTracks, playlistID)
	}

}

func swapLikedSongs(client *spotify.Client, alltracks []spotify.ID, playlistID spotify.ID) {
	client.AddTracksToLibrary()
	client.RemoveTracksFromLibrary()
}

func swapPlaylist(client *spotify.Client, alltracks []spotify.ID, playlistID spotify.ID) {

	partialTracks := alltracks[0:100]

	client.ReplacePlaylistTracks(context.Background(), playlistID, partialTracks...)

	i := len(partialTracks)
	for i < len(alltracks) {
		partialTracks = alltracks[i-1 : i+100]
		_, err := client.AddTracksToPlaylist(context.Background(), playlistID, partialTracks...)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func shuffle(alltracks []spotify.ID) []spotify.ID {
	for range 10 {
		for i := range alltracks {
			swapPos := rand.Intn(len(alltracks))
			alltracks[i], alltracks[swapPos] = alltracks[swapPos], alltracks[i]
		}
	}
	return alltracks
}

func getPlaylistTracks(client *spotify.Client, playlistID spotify.ID) []spotify.ID {
	playlist, err := client.GetPlaylist(context.Background(), playlistID)
	if err != nil {
		log.Fatal(err)
	}
	var allTracks []spotify.ID

	for _, track := range playlist.Tracks.Tracks {
		//fmt.Println(track.Track.ID)
		allTracks = append(allTracks, track.Track.ID)
	}

	return allTracks
}

func getSavedTracks(client *spotify.Client) []spotify.ID {

	offset := 0
	country := "US"
	limit := 50

	var allTracks []spotify.ID //ids of saved tracks
	saved, err := client.CurrentUsersTracks(context.Background(),
		spotify.Limit(limit), spotify.Country(country), spotify.Offset(offset))
	if err != nil {
		log.Fatal(err)
	}

	for _, track := range saved.Tracks {
		allTracks = append(allTracks, track.FullTrack.ID)
	}
	offset += limit

	for len(allTracks) < int(saved.Total) {
		fmt.Printf("saved %d songs\n", len(allTracks))
		saved, err := client.CurrentUsersTracks(context.Background(),
			spotify.Limit(limit), spotify.Country(country), spotify.Offset(offset))

		if err != nil {
			log.Fatal(err)
		}
		for _, track := range saved.Tracks {
			allTracks = append(allTracks, track.FullTrack.ID)
		}
		offset += limit
	}

	return allTracks
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	// use the token to get an authenticated client
	client := spotify.New(auth.Client(r.Context(), tok))
	fmt.Fprintf(w, "Login Completed!")
	ch <- client
}
