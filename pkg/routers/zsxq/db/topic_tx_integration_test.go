package db

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openTestDB 连接由 ZSXQ_TEST_DATABASE_URL 指定的临时 Postgres；未设置则跳过（对齐 tombkeeper
// 集成测试约定：只有 Postgres 才能验真实事务回滚，本仓无 sqlite 驱动）。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ZSXQ_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ZSXQ_TEST_DATABASE_URL to run the Postgres integration test")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return gdb
}

func dropZsxqTables(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	for _, table := range []string{"zsxq_topic", "zsxq_object", "zsxq_article", "zsxq_author"} {
		require.NoError(t, gdb.Exec("DROP TABLE IF EXISTS "+table).Error)
	}
}

func count(t *testing.T, gdb *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, gdb.Model(model).Count(&n).Error)
	return n
}

// TestSaveTopicTxAtomic 验证 SaveTopicTx 单事务原子性（决策 4）：根行最后写，中途失败整体回滚。
func TestSaveTopicTxAtomic(t *testing.T) {
	t.Run("first crawl rolls back with no visible rows when root save fails", func(t *testing.T) {
		gdb := openTestDB(t)
		dropZsxqTables(t, gdb)
		t.Cleanup(func() { dropZsxqTables(t, gdb) })
		// 只建侧表，故意不建 zsxq_topic —— 根行写入必失败。
		require.NoError(t, gdb.AutoMigrate(&Author{}, &Article{}, &Object{}))
		store := NewDBService(gdb)

		root := &Topic{ID: 100, Time: time.Now(), GroupID: 5, Type: "talk", AuthorID: 7, Raw: []byte("{}")}
		author := &Author{ID: 7, Name: "作者"}
		article := &Article{ID: "art-1", Title: "外部文章", Raw: []byte("<p>x</p>")}
		objects := []Object{{ID: 21, TopicID: 100, Type: "image", ObjectKey: "zsxq/21.jpg"}}

		err := store.SaveTopicTx(root, author, article, objects)
		require.Error(t, err, "root 表缺失应导致事务失败")

		assert.Zero(t, count(t, gdb, &Author{}), "作者应随事务回滚")
		assert.Zero(t, count(t, gdb, &Article{}), "外部文章应随事务回滚")
		assert.Zero(t, count(t, gdb, &Object{}), "对象应随事务回滚")
	})

	t.Run("update keeps prior rows intact when root save fails", func(t *testing.T) {
		gdb := openTestDB(t)
		dropZsxqTables(t, gdb)
		t.Cleanup(func() { dropZsxqTables(t, gdb) })
		require.NoError(t, gdb.AutoMigrate(&Topic{}, &Author{}, &Article{}, &Object{}))
		store := NewDBService(gdb)

		// 预置一次成功抓取的旧行。
		require.NoError(t, store.SaveTopicTx(
			&Topic{ID: 100, Time: time.Now(), GroupID: 5, Type: "talk", AuthorID: 1, Raw: []byte("{}")},
			&Author{ID: 1, Name: "旧名"},
			nil,
			[]Object{{ID: 10, TopicID: 100, Type: "image", ObjectKey: "zsxq/10.jpg"}},
		))

		// 破坏 root 写入：删掉 raw 列，后续 SaveTopicTx 的根行 INSERT 必失败。
		require.NoError(t, gdb.Exec("ALTER TABLE zsxq_topic DROP COLUMN raw").Error)

		err := store.SaveTopicTx(
			&Topic{ID: 200, Time: time.Now(), GroupID: 5, Type: "talk", AuthorID: 1, Raw: []byte("{}")},
			&Author{ID: 1, Name: "新名"}, // upsert 同一作者，若提交会覆盖旧名
			nil,
			[]Object{{ID: 20, TopicID: 200, Type: "image", ObjectKey: "zsxq/20.jpg"}},
		)
		require.Error(t, err)

		var author Author
		require.NoError(t, gdb.Where("id = ?", 1).First(&author).Error)
		assert.Equal(t, "旧名", author.Name, "作者 upsert 应随事务回滚，保留旧名")

		assert.EqualValues(t, 1, count(t, gdb, &Object{}), "新对象应回滚，仅剩旧对象")
		assert.EqualValues(t, 1, count(t, gdb, &Topic{}), "新根行应回滚，仅剩旧根行")
	})
}
