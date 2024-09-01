package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	Token        = "1990679231:AAGgPkitpScXGJv9WMBN6qiHJaSHwVXaN64"
	DownloadPath = "./downloads"
)

var userVideos = make(map[int64][]string)

func main() {
	bot, err := tgbotapi.NewBotAPI(Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			switch update.Message.Text {
			case "/start":
				userVideos[update.Message.Chat.ID] = []string{}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Начинаем запись видеосообщений. Отправьте /merge для объединения.")
				bot.Send(msg)
			case "/merge":
				if videos, ok := userVideos[update.Message.Chat.ID]; ok && len(videos) > 0 {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Начинаем объединение видеосообщений. Это может занять некоторое время.")
					bot.Send(msg)
					mergedFilePath, err := mergeVideos(videos)
					if err != nil {
						log.Printf("Error merging videos: %v", err)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ошибка при объединении видеосообщений.")
						bot.Send(msg)
					} else {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Видеосообщения объединены.")
						bot.Send(msg)
						videoMsg := tgbotapi.NewVideo(update.Message.Chat.ID, tgbotapi.FilePath(mergedFilePath))
						bot.Send(videoMsg)
					}
					delete(userVideos, update.Message.Chat.ID)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет видеосообщений для объединения.")
					bot.Send(msg)
				}
			default:
				if update.Message.VideoNote != nil {
					videoNote := update.Message.VideoNote
					fileID := videoNote.FileID

					file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
					if err != nil {
						log.Println("Error getting file:", err)
						continue
					}

					filePath := filepath.Join(DownloadPath, fmt.Sprintf("%s.mp4", fileID))
					err = downloadFile(file.Link(bot.Token), filePath)
					if err != nil {
						log.Println("Error downloading file:", err)
						continue
					}

					userVideos[update.Message.Chat.ID] = append(userVideos[update.Message.Chat.ID], filePath)
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Видеосообщение сохранено как %s", filePath))
					bot.Send(msg)
				}
			}
		}
	}
}

func downloadFile(url string, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := os.Stat(DownloadPath); os.IsNotExist(err) {
		os.MkdirAll(DownloadPath, os.ModePerm)
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func mergeVideos(videoPaths []string) (string, error) {
	mergedFilePath := filepath.Join(DownloadPath, "merged.mp4")
	listFilePath := filepath.Join(DownloadPath, "videos.txt")

	listFile, err := os.Create(listFilePath)
	if err != nil {
		return "", err
	}
	defer listFile.Close()

	for _, videoPath := range videoPaths {
		absPath, err := filepath.Abs(videoPath)
		if err != nil {
			return "", err
		}
		_, err = listFile.WriteString(fmt.Sprintf("file '%s'\n", absPath))
		if err != nil {
			return "", err
		}
	}

	log.Println("Starting video merge process...")
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", listFilePath, "-c", "copy", mergedFilePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		log.Printf("Error merging videos: %v, %s", err, stderr.String())
		return "", err
	}
	log.Println("Video merge process completed.")

	return mergedFilePath, nil
}
