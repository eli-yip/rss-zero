package apiModels

type (
	ZvideoList struct {
		Paging Paging   `json:"paging"`
		Data   []Zvideo `json:"data"`
	}

	Zvideo struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Video       Video  `json:"video"`
		PublishedAt int    `json:"published_at"` // 1661742626
	}

	Video struct {
		VideoID    string                   `json:"video_id"`
		PlayList   *map[string]PlayListInfo `json:"playlist"`
		PlayListV2 *map[string]PlayListInfo `json:"playlist_v2"`
	}

	PlayListInfo struct {
		PlayUrl string  `json:"play_url"`
		Bitrate float64 `json:"bitrate"`
	}
)
