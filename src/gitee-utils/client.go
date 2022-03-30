package gitee_utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	sdk "gitee.com/openeuler/go-gitee/gitee"
	"github.com/antihax/optional"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type Label struct {
	Name       string `json:"labelName"`
	Desciption string `json:"labelDesciption"`
}

type User struct {
	IssueUserName string `json:"issueUserName"`
	IssueUserID   string `json:"issueUserID"`
	IsEntUser     int    `json:"isEntUser"`
	IsOrgUser     int    `json:"isOrgUser"`
}

type RepoInfo struct {
	Org  string `json:"org"`
	Repo string `json:"repo"`
	Ent  string `json:"ent"`
}

type Issue struct {
	IssueID         string   `json:"issueID"`
	IssueAction     string   `json:"issueAction"`
	IssueUser       User     `json:"issueUser"`
	IssueTime       string   `json:"issueTime"`
	IssueUpdateTime string   `json:"issueUpdateTime"`
	IssueAssignee   string   `json:"issueAssignee"`
	IssueLabel      []Label  `json:"issueLabel"`
	IssueTitle      string   `json:"issueTitle"`
	IssueContent    string   `json:"issueContent"`
	RepoInfo        RepoInfo `json:"repoInfo"`
}

var _ Client = (*client)(nil)

type client struct {
	ac *sdk.APIClient

	mut      sync.Mutex // protects botName and email
	userData *sdk.User
}

func NewClient(getToken func() []byte) Client {
	token := string(getToken())

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	conf := sdk.NewConfiguration()
	conf.HTTPClient = oauth2.NewClient(context.Background(), ts)

	c := sdk.NewAPIClient(conf)
	return &client{ac: c}
}

// BotName returns the login of the authenticated identity.
func (c *client) BotName() (string, error) {
	err := c.getUserData()
	if err != nil {
		return "", err
	}

	return c.userData.Login, nil
}

func (c *client) Email() (string, error) {
	err := c.getUserData()
	if err != nil {
		return "", err
	}

	return c.userData.Email, nil
}

func (c *client) CreatePullRequest(org, repo, title, body, head, base string, canModify bool) (sdk.PullRequest, error) {
	opts := sdk.CreatePullRequestParam{
		Title:             title,
		Head:              head,
		Base:              base,
		Body:              body,
		PruneSourceBranch: true,
	}

	pr, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPulls(
		context.Background(), org, repo, opts)

	return pr, formatErr(err, "create pull request")
}

func (c *client) GetPullRequests(org, repo string, opts ListPullRequestOpt) ([]sdk.PullRequest, error) {

	setStr := func(t *optional.String, v string) {
		if v != "" {
			*t = optional.NewString(v)
		}
	}
	opt := sdk.GetV5ReposOwnerRepoPullsOpts{}
	setStr(&opt.State, opts.State)
	setStr(&opt.Head, opts.Head)
	setStr(&opt.Base, opts.Base)
	setStr(&opt.Sort, opts.Sort)
	setStr(&opt.Direction, opts.Direction)
	if opts.MilestoneNumber > 0 {
		opt.MilestoneNumber = optional.NewInt32(int32(opts.MilestoneNumber))
	}
	if opts.Labels != nil && len(opts.Labels) > 0 {
		opt.Labels = optional.NewString(strings.Join(opts.Labels, ","))
	}

	var r []sdk.PullRequest
	p := int32(1)
	for {
		opt.Page = optional.NewInt32(p)
		prs, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPulls(context.Background(), org, repo, &opt)
		if err != nil {
			return nil, formatErr(err, "get pull requests")
		}

		if len(prs) == 0 {
			break
		}

		r = append(r, prs...)
		p++
	}

	return r, nil
}

func (c *client) UpdatePullRequest(org, repo string, number int32, param sdk.PullRequestUpdateParam) (sdk.PullRequest, error) {
	pr, _, err := c.ac.PullRequestsApi.PatchV5ReposOwnerRepoPullsNumber(context.Background(), org, repo, number, param)
	return pr, formatErr(err, "update pull request")
}

func (c *client) GetGiteePullRequest(org, repo string, number int) (sdk.PullRequest, error) {
	pr, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumber(
		context.Background(), org, repo, int32(number), nil)
	return pr, formatErr(err, "get pull request")
}

func (c *client) getUserData() error {
	if c.userData == nil {
		c.mut.Lock()
		defer c.mut.Unlock()

		if c.userData == nil {
			u, _, err := c.ac.UsersApi.GetV5User(context.Background(), nil)
			if err != nil {
				return formatErr(err, "fetch bot name")
			}
			c.userData = &u
		}
	}
	return nil
}

func (c *client) GetRef(org, repo, ref string) (string, error) {
	branch := strings.TrimPrefix(ref, "heads/")
	b, _, err := c.ac.RepositoriesApi.GetV5ReposOwnerRepoBranchesBranch(context.Background(), org, repo, branch, nil)
	if err != nil {
		return "", formatErr(err, "get branch")
	}

	return b.Commit.Sha, nil
}

func (c *client) GetPRLabels(org, repo string, number int) ([]sdk.Label, error) {
	var r []sdk.Label

	p := int32(1)
	opt := sdk.GetV5ReposOwnerRepoPullsNumberLabelsOpts{}
	for {
		opt.Page = optional.NewInt32(p)
		ls, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberLabels(
			context.Background(), org, repo, int32(number), &opt)
		if err != nil {
			return nil, formatErr(err, "list labels of pr")
		}

		if len(ls) == 0 {
			break
		}

		r = append(r, ls...)
		p++
	}

	return r, nil
}

func (c *client) ListPRComments(org, repo string, number int) ([]sdk.PullRequestComments, error) {
	var r []sdk.PullRequestComments

	p := int32(1)
	opt := sdk.GetV5ReposOwnerRepoPullsNumberCommentsOpts{}
	for {
		opt.Page = optional.NewInt32(p)
		cs, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberComments(
			context.Background(), org, repo, int32(number), &opt)
		if err != nil {
			return nil, formatErr(err, "list comments of pr")
		}

		if len(cs) == 0 {
			break
		}

		r = append(r, cs...)
		p++
	}

	return r, nil
}

func (c *client) ListPrIssues(org, repo string, number int32) ([]sdk.Issue, error) {
	var issues []sdk.Issue
	p := int32(1)
	opt := sdk.GetV5ReposOwnerRepoPullsNumberIssuesOpts{}
	for {
		opt.Page = optional.NewInt32(p)
		iss, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberIssues(context.Background(), org, repo, number, &opt)
		if err != nil {
			return nil, formatErr(err, "list issues of pr")
		}
		if len(iss) == 0 {
			break
		}
		issues = append(issues, iss...)
		p++
	}
	return issues, nil
}

func (c *client) DeletePRComment(org, repo string, ID int) error {
	_, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsCommentsId(
		context.Background(), org, repo, int32(ID), nil)
	return formatErr(err, "delete comment of pr")
}

func (c *client) CreatePRComment(org, repo string, number int, comment string) error {
	opt := sdk.PullRequestCommentPostParam{Body: comment}
	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberComments(
		context.Background(), org, repo, int32(number), opt)
	return formatErr(err, "create comment of pr")
}

func (c *client) UpdatePRComment(org, repo string, commentID int, comment string) error {
	opt := sdk.PullRequestCommentPatchParam{Body: comment}
	_, _, err := c.ac.PullRequestsApi.PatchV5ReposOwnerRepoPullsCommentsId(
		context.Background(), org, repo, int32(commentID), opt)
	return formatErr(err, "update comment of pr")
}

func (c *client) AddPRLabel(org, repo string, number int, labels []string) error {
	opt := sdk.PullRequestLabelPostParam{Body: labels}
	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberLabels(
		context.Background(), org, repo, int32(number), opt)
	return formatErr(err, "add labels for pr")
}

func (c *client) RemovePRLabel(org, repo string, number int, label string) error {
	// gitee-utils's bug, it can't deal with the label which includes '/'
	label = strings.Replace(label, "/", "%2F", -1)

	_, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsLabel(
		context.Background(), org, repo, int32(number), label, nil)
	return formatErr(err, "remove label of pr")
}

func (c *client) AssignPR(org, repo string, number int, logins []string) error {
	opt := sdk.PullRequestAssigneePostParam{Assignees: strings.Join(logins, ",")}

	_, _, err := c.ac.PullRequestsApi.PostV5ReposOwnerRepoPullsNumberAssignees(
		context.Background(), org, repo, int32(number), opt)
	return formatErr(err, "assign reviewer to pr")
}

func (c *client) UnassignPR(org, repo string, number int, logins []string) error {
	_, _, err := c.ac.PullRequestsApi.DeleteV5ReposOwnerRepoPullsNumberAssignees(
		context.Background(), org, repo, int32(number), strings.Join(logins, ","), nil)
	return formatErr(err, "unassign reviewer from pr")
}

func (c *client) GetPRCommits(org, repo string, number int) ([]sdk.PullRequestCommits, error) {
	commits, _, err := c.ac.PullRequestsApi.GetV5ReposOwnerRepoPullsNumberCommits(
		context.Background(), org, repo, int32(number), nil)
	return commits, formatErr(err, "get pr commits")
}

func (c *client) AssignGiteeIssue(org, repo, labels string, number string, login string) error {
	var labelStr []string
	labelsToAddStr := ""

	ll, _, errll := c.ac.LabelsApi.GetV5ReposOwnerRepoLabels(context.Background(), org, repo, nil)

	if errll != nil {
		return formatErr(errll, "fail to get labels")
	}

	labelsArr := strings.Split(labels, ",")

	for _, label := range labelsArr {
		for _, l := range ll {
			if label == l.Name {
				labelStr = append(labelStr, label)
			}
		}
	}

	if len(labelStr) == 0 {
		return formatErr(nil, "assign assignee to issue")
	}

	labelsToAddStr = strings.Join(labelStr, ",")

	opt := sdk.IssueUpdateParam{
		Repo:     repo,
		Assignee: login,
		Labels:   labelsToAddStr,
	}

	_, v, err := c.ac.IssuesApi.PatchV5ReposOwnerIssuesNumber(
		context.Background(), org, number, opt)

	if err != nil {
		if v.StatusCode == 403 {
			return ErrorForbidden{err: formatErr(err, "assign assignee to issue").Error()}
		}
	}
	return formatErr(err, "assign assignee to issue")
}

func (c *client) UnassignGiteeIssue(org, repo, labels string, number string, login string) error {
	return c.AssignGiteeIssue(org, repo, labels, number, " ")
}

func (c *client) CreateGiteeIssueComment(org, repo string, number string, comment string) error {
	opt := sdk.IssueCommentPostParam{Body: comment}
	_, _, err := c.ac.IssuesApi.PostV5ReposOwnerRepoIssuesNumberComments(
		context.Background(), org, repo, number, opt)
	return formatErr(err, "create issue comment")
}

func (c *client) DeleteGiteeIssueComment(org, repo string, ID int) error {
	_, err := c.ac.IssuesApi.DeleteV5ReposOwnerRepoIssuesCommentsId(
		context.Background(), org, repo, int32(ID), nil)
	return formatErr(err, "delete comment of issue")
}

func (c *client) UpdateGiteeIssueComment(org, repo string, commentID int, comment string) error {
	_, _, err := c.ac.IssuesApi.PatchV5ReposOwnerRepoIssuesCommentsId(
		context.Background(), org, repo, int32(commentID), comment, nil)
	return formatErr(err, "update comment of issue")
}

func (c *client) IsCollaborator(owner, repo, login string) (bool, error) {
	v, err := c.ac.RepositoriesApi.GetV5ReposOwnerRepoCollaboratorsUsername(
		context.Background(), owner, repo, login, nil)
	if err != nil {
		if v.StatusCode == 404 {
			return false, nil
		}
		return false, formatErr(err, "get collaborator of pr")
	}
	return true, nil
}

func (c *client) IsMember(org, login string) (bool, error) {
	_, v, err := c.ac.OrganizationsApi.GetV5OrgsOrgMembershipsUsername(
		context.Background(), org, login, nil)
	if err != nil {
		if v.StatusCode == 404 {
			return false, nil
		}
		return false, formatErr(err, "get member of org")
	}
	return true, nil
}

func (c *client) GetGiteeRepo(org, repo string) (sdk.Project, error) {
	v, _, err := c.ac.RepositoriesApi.GetV5ReposOwnerRepo(context.Background(), org, repo, nil)
	return v, formatErr(err, "get repo")
}

func (c *client) MergePR(owner, repo string, number int, opt sdk.PullRequestMergePutParam) error {
	_, err := c.ac.PullRequestsApi.PutV5ReposOwnerRepoPullsNumberMerge(
		context.Background(), owner, repo, int32(number), opt)
	return formatErr(err, "merge pr")
}

func (c *client) GetRepos(org string) ([]sdk.Project, error) {
	opt := sdk.GetV5OrgsOrgReposOpts{}
	var r []sdk.Project
	p := int32(1)
	for {
		opt.Page = optional.NewInt32(p)
		ps, _, err := c.ac.RepositoriesApi.GetV5OrgsOrgRepos(context.Background(), org, &opt)
		if err != nil {
			return nil, formatErr(err, "list repos")
		}

		if len(ps) == 0 {
			break
		}
		r = append(r, ps...)
		p++
	}
	return r, nil
}

func (c *client) AddIssueLabel(org, repo, number string, label []string) error {
	opt := sdk.PullRequestLabelPostParam{Body: label}
	_, _, err := c.ac.LabelsApi.PostV5ReposOwnerRepoIssuesNumberLabels(context.Background(), org, repo, number, opt)
	return formatErr(err, "add issue label")
}

func (c *client) RemoveIssueLabel(org, repo, number, label string) error {
	label = strings.Replace(label, "/", "%2F", -1)
	_, err := c.ac.LabelsApi.DeleteV5ReposOwnerRepoIssuesNumberLabelsName(
		context.Background(), org, repo, number, label, nil)
	return formatErr(err, "rm issue label")
}

func (c *client) AddIssueAssignee(org, repo, number, token, assignee string) error {
	updateParam := sdk.IssueUpdateParam{AccessToken: token, Repo: repo, Assignee: assignee}
	_, _, err := c.ac.IssuesApi.PatchV5ReposOwnerIssuesNumber(context.Background(), org, number, updateParam)
	return formatErr(err, "issue update")
}

func (c *client) GetUserOrg(login string) ([]sdk.Group, error) {
	group, _, err := c.ac.OrganizationsApi.GetV5UsersUsernameOrgs(context.Background(), login, nil)
	return group, formatErr(err, "get org")
}

func (c *client) GetUserEnt(ent, login string) (sdk.EnterpriseMember, error) {
	member, _, err := c.ac.EnterprisesApi.GetV5EnterprisesEnterpriseMembersUsername(context.Background(), ent, login, nil)
	return member, formatErr(err, "get ent")
}

func (c *client) ListIssues(owner, repo, state, since, createAt string, page, perPage int) ([]sdk.Issue, *http.Response, error) {
	oState := optional.NewString(state)
	oPage := optional.NewInt32(int32(page))
	oPerPage := optional.NewInt32(int32(perPage))
	oSince := optional.NewString(since)
	oCreateAt := optional.NewString(createAt)
	issueParam := sdk.GetV5ReposOwnerRepoIssuesOpts{State: oState, Page: oPage, PerPage: oPerPage, Since: oSince, CreatedAt: oCreateAt}
	issues, res, err := c.ac.IssuesApi.GetV5ReposOwnerRepoIssues(context.Background(), owner, repo, &issueParam)
	return issues, res, formatErr(err, "list issues")
}

func (c *client) ListIssuesA(owner, repo, state, createAt string, page, perPage int) ([]sdk.Issue, *http.Response, error) {
	oState := optional.NewString(state)
	oPage := optional.NewInt32(int32(page))
	oPerPage := optional.NewInt32(int32(perPage))
	oCreateAt := optional.NewString(createAt)
	issueParam := sdk.GetV5ReposOwnerRepoIssuesOpts{State: oState, Page: oPage, PerPage: oPerPage, CreatedAt: oCreateAt}
	issues, res, err := c.ac.IssuesApi.GetV5ReposOwnerRepoIssues(context.Background(), owner, repo, &issueParam)
	return issues, res, formatErr(err, "list issues")
}

func (c *client) ListLabels(owner, repo string) ([]sdk.Label, error) {
	labels, _, err := c.ac.LabelsApi.GetV5ReposOwnerRepoLabels(context.Background(), owner, repo, nil)
	return labels, formatErr(err, "list labels")
}

func (c *client) GetRecommendation(labels string) (string, error) {
	// create path and map variables
	urlValues := url.Values{}
	urlValues.Add("labels", labels)
	resp, err := http.PostForm("http://34.92.52.47/predict", urlValues)
	if err != nil {
		// handle error
		formatErr(err, "request error")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// handle error
		formatErr(err, "request error")
	}
	participants := string(body[:])
	return participants, formatErr(err, "GetRecommendation")
}

func (c *client) SendIssue(issue Issue, apiUrl string) (string, error) {
	// create path and map variables
	var retries = 3
	var post, err = json.Marshal(issue)
	if err != nil {
		return "Bad Json", err
	}

	var jsonStr = []byte(post)

	req, err := http.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonStr))
	if err != nil {
		return "Bad Request", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	for retries > 0 {
		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
			LogInstance.WithFields(logrus.Fields{
				"context": "Send issue error, please check whether the apiUrl is active",
			}).Info("info log")
			retries -= 1
		} else {
			defer resp.Body.Close()
			fmt.Println("response Status:", resp.Status)
			fmt.Println("response Headers:", resp.Header)
			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Println("response Body:", string(body))
			break
		}
	}

	return "Good Request", nil
}

func formatErr(err error, doWhat string) error {
	if err == nil {
		return err
	}
	return fmt.Errorf("Failed to %s: %s", doWhat, err.Error())
}
