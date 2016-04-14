// slacktsbot by Daniel Aberger
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Payload struct {
	Channel   string `json:"channel"`
	IconEmoji string `json:"icon_emoji"`
	Text      string `json:"text"`
	Username  string `json:"username"`
}

var (
	users = map[string]bool{}

	port         *string = flag.String("port", "12345", "teamspeak 3 query port number")
	host         *string = flag.String("host", "your.ts-server.com", "teamspeak 3 server host")
	user         *string = flag.String("user", "youradminuser", "teamspeak 3 server admin user")
	pass         *string = flag.String("password", "yourpassword", "teamspeak 3 server admin user password")
	slackhookurl *string = flag.String("slack", "https://hooks.slack.com/services/YOURSLACKWEBHOOKURL", "slack webhook url")
)

func init() {
	// ./slacktsbot -port=12345 -host=your.ts-server.com -user=yourusername -pass=yourpassword
	flag.Parse()
}
func main() {

	for {
		var err error
		users, err = getUsers()
		if err != nil {
			log.Println(err)
			log.Println("Trying again in 60 seconds.")
			time.Sleep(60 * time.Second)
		} else {
			break
		}
	}

	for {
		neu, err := getUsers()
		if err != nil {
			log.Println(err)
			time.Sleep(50 * time.Second)
		} else {
			for i, _ := range neu {
				if users[i] != neu[i] {
					generateAndSendPayload(i, "join")
				}
			}
			for i, _ := range users {
				if users[i] != neu[i] {
					generateAndSendPayload(i, "leave")
				}
			}
			users = neu
		}

		time.Sleep(10 * time.Second)
	}
}

func generateAndSendPayload(user string, action string) {
	payload := Payload{
		Channel:   "#wgs-general",
		IconEmoji: ":poop:",
		Text:      "",
		Username:  "TS3 Bot",
	}
	var actiontext string
	if action == "join" {
		actiontext = " joined TS3."
	}
	if action == "leave" {
		actiontext = " left TS3."
	}
	timestamp := time.Now().Format(time.RFC822)
	payload.Text = timestamp + " " + user + actiontext
	b, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("error:", err)
	} else {
		sendToSlack(b)
	}
}

func sendToSlack(json []byte) {
	url := *slackhookurl
	log.Println("URL:>", url)

	var jsonStr = json
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	log.Println("response Status:", resp.Status)
	//log.Println("response Headers:", resp.Header)
	//body, _ := ioutil.ReadAll(resp.Body)
	//log.Println("response Body:", string(body))
}

func getUsers() (u map[string]bool, e error) {
	log.Println("Querying Teamspeak 3 server '" + *host + ":" + *port + "'...")
	conn, err := net.Dial("tcp", *host+":"+*port)

	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for i := 0; i < 2; i++ { // get rid of banner
		_, _, err := reader.ReadLine()
		if err != nil {
			if err.Error() == "EOF" {
				log.Println("Connection closed by foreign host.")
			}
			return nil, err
		}
	}
	writer.WriteString("login " + *user + " " + *pass + "\n")
	writer.Flush()
	_, _, err = reader.ReadLine() // get rid of "OK" message
	if err != nil {
		if err.Error() == "EOF" {
			log.Println("Connection closed by foreign host.")
		}
		return nil, err
	}

	writer.WriteString("use sid=1\n")
	writer.Flush()
	_, _, err = reader.ReadLine() // get rid of "OK" message
	if err != nil {
		if err.Error() == "EOF" {
			log.Println("Connection closed by foreign host.")
		}
		return nil, err
	}

	writer.WriteString("clientlist\n")
	writer.Flush()
	line, _, err := reader.ReadLine() // get rid of "OK" message
	if err != nil {
		if err.Error() == "EOF" {
			log.Println("Connection closed by foreign host.")
		}
		return nil, err
	}
	users_raw := string(line)

	re := regexp.MustCompile("client_nickname=(.*?) ")
	users_still_raw := re.FindAllString(users_raw, -1)

	for i := range users_still_raw {
		users_still_raw[i] = strings.Trim(users_still_raw[i], " ")
		users_still_raw[i] = strings.Replace(users_still_raw[i], "client_nickname=", "", 1)
		users_still_raw[i] = strings.Replace(users_still_raw[i], "\\s", " ", -1)
	}
	users := make([]string, 0)
	for i := range users_still_raw {
		if !strings.HasPrefix(users_still_raw[i], "serveradmin from") {
			users = append(users, users_still_raw[i])
		}
	}
	result := make(map[string]bool)
	for i := range users {
		result[users[i]] = true
	}
	conn.Close()
	return result, nil
}
