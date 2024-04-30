package happeninghound

import "happeninghound/client"

func main() {

	// 起動
	// configファイルロード＆チェック
	// slack serviceインスタンス生成
	// gdrive serviceインスタンス生成
	// botに渡して実行？
	client.Run()
}
