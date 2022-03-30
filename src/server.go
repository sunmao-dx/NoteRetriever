package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"io/ioutil"

	"gitee.com/openeuler/go-gitee/gitee"
	gitee_utils "gitee.com/sunmao-dx/strategy-executor/src/gitee-utils"
	"github.com/sirupsen/logrus"
)

var repo []byte

func getToken() []byte {
	return []byte(os.Getenv("gitee_token"))
	// return []byte("6be0cf40358beb15cf17b7e63b7a576e")
	// return []byte("51c7d200177659ac3f2cecab7c083c43")

}

func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Event received.")

	// // Loop over header names
	// for name, values := range r.Header {
	// 	// Loop over all values for the name.
	// 	for _, value := range values {
	// 		fmt.Println(name, value)
	// 	}
	// }
	// fmt.Println(r.Body)

	defer r.Body.Close()
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "gitee hook is broken",
		}).Info("info log")
		return
	}

	var ic gitee.NoteEvent
	if err := json.Unmarshal(payload, &ic); err != nil {
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": "gitee hook is broken",
		}).Info("info log")
		return
	}
	gitee_utils.LogInstance.WithFields(logrus.Fields{
		"context": "gitee hook success",
	}).Info("info log")

	go handleCommentEvent(&ic)
}

func handleCommentEvent(i *gitee.NoteEvent) {
	switch *(i.NoteableType) {
	case "Issue":
		go handleIssueCommentEvent(i)
	default:
		return
	}
}

func handleIssueCommentEvent(i *gitee.NoteEvent) {
	return
}

/*
func getLabels(initLabels []gitee.LabelHook) []gitee_utils.Label {
	var issueLabel gitee_utils.Label
	var issueLabels []gitee_utils.Label
	for _, label := range initLabels {
		issueLabel.Name = label.Name
		issueLabel.Desciption = label.Name
		issueLabels = append(issueLabels, issueLabel)
	}
	return issueLabels
}

func isUserInEnt(login, entOrigin string, c gitee_utils.Client) int {
	_, err := c.GetUserEnt(entOrigin, login)
	if err != nil && !strings.Contains(err.Error(), "timeout") {
		fmt.Println(err.Error() + login + " is not an Ent memeber")
		gitee_utils.LogInstance.WithFields(logrus.Fields{
			"context": err.Error() + " " + login + " is not an Ent memeber",
		}).Info("info log")
		return 0
	} else {
		if err == nil {
			fmt.Println(" is an Ent memeber")
			gitee_utils.LogInstance.WithFields(logrus.Fields{
				"context": login + " is an Ent memeber",
			}).Info("info log")
			return 1
		} else {
			fmt.Println(err.Error() + "  now, retry...")
			gitee_utils.LogInstance.WithFields(logrus.Fields{
				"context": "  now, retry...",
			}).Info("info log")
			time.Sleep(time.Duration(5) * time.Second)
			return isUserInEnt(login, entOrigin, c)
		}
	}
}

func _init(i gitee_utils.Issue) gitee_utils.Issue {
	i.IssueID = "XXXXXX"
	i.IssueAction = "Open"
	i.IssueUser.IssueUserID = "no_name"
	i.IssueUser.IssueUserName = "NO_NAME"
	i.IssueUser.IsOrgUser = 0
	i.IssueUser.IsEntUser = 1
	i.IssueAssignee = "no_assignee"
	i.IssueLabel = nil

	i.IssueTime = time.Now().Format(time.RFC3339)
	i.IssueUpdateTime = time.Now().Format(time.RFC3339)
	i.IssueTitle = "no_title"
	i.IssueContent = "no_content"
	return i
}
*/

func main() {
	http.HandleFunc("/", ServeHTTP)
	http.ListenAndServe(":8002", nil)
}

// $ echo "export GO111MODULE=on" >> ~/.bashrc
// $ echo "export GOPROXY=https://goproxy.cn" >> ~/.bashrc
// $ source ~/.bashrc
