package main

import (
	"context"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-github/v38/github"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type RepoInfo struct {
	Owner            string
	Repo             string
	LatestCommitTime string
	TotalCommits     int
	Stars            int
	Forks            int
	Watchers         int
	UsedBy           int
	Contributors     int
	PullRequests     int
}

func main() {
	// 加载 .env 文件
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	// 从 .env 文件中获取 GitHub token
	token := os.Getenv("GITHUB_TOKEN")
	var client *github.Client
	if token != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
		log.Println("Warning: GITHUB_TOKEN not set. Using unauthenticated client.")
	}

	// 获取 GitHub 项目列表
	repos, err := scrapeGitHubRepos("https://core.telegram.org/bots/samples")
	if err != nil {
		log.Fatalf("Error scraping GitHub repos: %v", err)
	}

	// 获取每个仓库的信息
	var repoInfos []RepoInfo
	for _, repo := range repos {
		info, err := getRepoInfo(client, repo.Owner, repo.Repo)
		if err != nil {
			log.Printf("Error getting info for %s/%s: %v", repo.Owner, repo.Repo, err)
			continue
		}
		repoInfos = append(repoInfos, info)
	}

	// 打印 Markdown 表格
	printMarkdownTable(repoInfos)
}

func scrapeGitHubRepos(url string) ([]RepoInfo, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var repos []RepoInfo
	doc.Find("a[href^='https://github.com/']").Each(func(i int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		parts := strings.Split(strings.TrimPrefix(href, "https://github.com/"), "/")
		if len(parts) >= 2 {
			repos = append(repos, RepoInfo{Owner: parts[0], Repo: parts[1]})
		}
	})

	return repos, nil
}

func getRepoInfo(client *github.Client, owner, repo string) (RepoInfo, error) {
	ctx := context.Background()
	info := RepoInfo{Owner: owner, Repo: repo}

	repoData, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return info, err
	}

	// 获取最新提交和总提交次数
	opt := &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: 1},
	}
	commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, opt)
	if err != nil || len(commits) == 0 {
		return info, err
	}
	info.LatestCommitTime = commits[0].Commit.Author.GetDate().Format("2006-01-02")

	// 获取总提交次数
	totalCommits := 0
	opt.ListOptions.PerPage = 100
	for {
		commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opt)
		if err != nil {
			return info, err
		}
		totalCommits += len(commits)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	info.TotalCommits = totalCommits

	contributors, _, err := client.Repositories.ListContributors(ctx, owner, repo, &github.ListContributorsOptions{Anon: "true"})
	if err != nil {
		return info, err
	}

	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State: "all"})
	if err != nil {
		return info, err
	}

	usedBy, err := scrapeUsedByCount(fmt.Sprintf("https://github.com/%s/%s", owner, repo))
	if err != nil {
		log.Printf("Error getting Used by count for %s/%s: %v", owner, repo, err)
		// 设置 UsedBy 为 0 而不是返回错误
		usedBy = 0
	}

	info.Stars = repoData.GetStargazersCount()
	info.Forks = repoData.GetForksCount()
	info.Watchers = repoData.GetSubscribersCount()
	info.UsedBy = usedBy
	info.Contributors = len(contributors)
	info.PullRequests = len(prs)

	return info, nil
}

func scrapeUsedByCount(url string) (int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, err
	}

	usedByText := doc.Find("a[href$='/network/dependents'] span.Counter").Text()
	usedByText = strings.TrimSpace(strings.ReplaceAll(usedByText, ",", ""))
	if usedByText == "" {
		// 如果没有找到 "Used by" 信息，返回 0 而不是错误
		return 0, nil
	}
	usedBy, err := strconv.Atoi(usedByText)
	if err != nil {
		return 0, err
	}

	return usedBy, nil
}

func printMarkdownTable(repoInfos []RepoInfo) {
	fmt.Println("| Repository | Latest Commit | Total Commits | Stars | Forks | Watchers | Used by | Contributors | Pull Requests |")
	fmt.Println("|------------|---------------|---------------|-------|-------|----------|---------|--------------|---------------|")
	for _, info := range repoInfos {
		fmt.Printf("| [%s/%s](https://github.com/%s/%s) | %s | %d | %d | %d | %d | %d | %d | %d |\n",
			info.Owner, info.Repo, info.Owner, info.Repo,
			info.LatestCommitTime, info.TotalCommits, info.Stars,
			info.Forks, info.Watchers, info.UsedBy,
			info.Contributors, info.PullRequests)
	}
}
