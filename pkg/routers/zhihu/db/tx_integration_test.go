package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// openTestDB 连接由 ZHIHU_TEST_DATABASE_URL 指定的临时 Postgres；未设置则跳过（对齐 zsxq /
// tombkeeper 集成测试约定：只有 Postgres 才能验真实事务回滚，本仓无 sqlite 驱动）。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("ZHIHU_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ZHIHU_TEST_DATABASE_URL to run the Postgres integration test")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return gdb
}

func dropZhihuTables(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	for _, table := range []string{"zhihu_answer", "zhihu_question", "zhihu_article", "zhihu_author", "zhihu_object", "zhihu_pin"} {
		require.NoError(t, gdb.Exec("DROP TABLE IF EXISTS "+table).Error)
	}
}

func count(t *testing.T, gdb *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, gdb.Model(model).Count(&n).Error)
	return n
}

// TestSaveAnswerTxAtomic 验证 SaveAnswerTx 单事务原子性（决策 4）：根行最后写，中途失败整体回滚。
func TestSaveAnswerTxAtomic(t *testing.T) {
	t.Run("first crawl rolls back with no visible rows when root save fails", func(t *testing.T) {
		gdb := openTestDB(t)
		dropZhihuTables(t, gdb)
		t.Cleanup(func() { dropZhihuTables(t, gdb) })
		// 只建侧表，故意不建 zhihu_answer —— 根行写入必失败。
		require.NoError(t, gdb.AutoMigrate(&Question{}, &Object{}))
		store := NewDBService(gdb)

		answer := &Answer{ID: 100, QuestionID: 640511134, AuthorID: "canglimo", Raw: []byte("{}"), Status: AnswerStatusCompleted}
		question := &Question{ID: 640511134, Title: "标题"}
		objects := []Object{{ID: 21, Type: ObjectTypeImage, ObjectKey: "zhihu/21.jpg"}}

		err := store.SaveAnswerTx(answer, question, objects)
		require.Error(t, err, "answer 根表缺失应导致事务失败")

		assert.Zero(t, count(t, gdb, &Question{}), "问题应随事务回滚")
		assert.Zero(t, count(t, gdb, &Object{}), "对象应随事务回滚")
	})

	t.Run("update keeps prior rows intact when root save fails", func(t *testing.T) {
		gdb := openTestDB(t)
		dropZhihuTables(t, gdb)
		t.Cleanup(func() { dropZhihuTables(t, gdb) })
		require.NoError(t, gdb.AutoMigrate(&Answer{}, &Question{}, &Object{}))
		store := NewDBService(gdb)

		// 预置一次成功抓取的旧行。
		require.NoError(t, store.SaveAnswerTx(
			&Answer{ID: 100, QuestionID: 1, AuthorID: "canglimo", Raw: []byte("{}"), Status: AnswerStatusCompleted},
			&Question{ID: 1, Title: "旧标题"},
			[]Object{{ID: 10, Type: ObjectTypeImage, ObjectKey: "zhihu/10.jpg"}},
		))

		// 破坏 root 写入：删掉 raw 列，后续 SaveAnswerTx 的根行 INSERT 必失败。
		require.NoError(t, gdb.Exec("ALTER TABLE zhihu_answer DROP COLUMN raw").Error)

		err := store.SaveAnswerTx(
			&Answer{ID: 200, QuestionID: 2, AuthorID: "canglimo", Raw: []byte("{}"), Status: AnswerStatusCompleted},
			&Question{ID: 2, Title: "新标题"},
			[]Object{{ID: 20, Type: ObjectTypeImage, ObjectKey: "zhihu/20.jpg"}},
		)
		require.Error(t, err)

		assert.EqualValues(t, 1, count(t, gdb, &Question{}), "新问题应回滚，仅剩旧问题")
		assert.EqualValues(t, 1, count(t, gdb, &Object{}), "新对象应回滚，仅剩旧对象")
		assert.EqualValues(t, 1, count(t, gdb, &Answer{}), "新根行应回滚，仅剩旧根行")
	})
}

// TestSaveArticleTxAtomic 验证 SaveArticleTx 根行最后写、中途失败整体回滚。
func TestSaveArticleTxAtomic(t *testing.T) {
	gdb := openTestDB(t)
	dropZhihuTables(t, gdb)
	t.Cleanup(func() { dropZhihuTables(t, gdb) })
	// 只建侧表，不建 zhihu_article —— 根行写入必失败。
	require.NoError(t, gdb.AutoMigrate(&Author{}, &Object{}))
	store := NewDBService(gdb)

	err := store.SaveArticleTx(
		&Article{ID: 300, AuthorID: "canglimo", Raw: []byte("{}")},
		&Author{ID: "canglimo", Name: "墨苍离"},
		[]Object{{ID: 31, Type: ObjectTypeImage, ObjectKey: "zhihu/31.jpg"}},
	)
	require.Error(t, err, "article 根表缺失应导致事务失败")

	assert.Zero(t, count(t, gdb, &Author{}), "作者应随事务回滚")
	assert.Zero(t, count(t, gdb, &Object{}), "对象应随事务回滚")
}

// TestSavePinTxAtomic 验证 SavePinTx 根行最后写、中途失败整体回滚（含一层 origin pin 根行）。
func TestSavePinTxAtomic(t *testing.T) {
	gdb := openTestDB(t)
	dropZhihuTables(t, gdb)
	t.Cleanup(func() { dropZhihuTables(t, gdb) })
	// 只建侧表，不建 zhihu_pin —— 根行写入必失败。
	require.NoError(t, gdb.AutoMigrate(&Author{}, &Object{}))
	store := NewDBService(gdb)

	// 两条 pin 根行（origin 在前、顶层在后）+ 作者 + 对象，任一根行失败整体回滚。
	err := store.SavePinTx(
		[]Pin{{ID: 41, AuthorID: "canglimo", Raw: []byte("{}")}, {ID: 42, AuthorID: "canglimo", Raw: []byte("{}")}},
		[]Author{{ID: "canglimo", Name: "墨苍离"}},
		[]Object{{ID: 43, Type: ObjectTypeImage, ObjectKey: "zhihu/43.jpg"}},
	)
	require.Error(t, err, "pin 根表缺失应导致事务失败")

	assert.Zero(t, count(t, gdb, &Author{}), "作者应随事务回滚")
	assert.Zero(t, count(t, gdb, &Object{}), "对象应随事务回滚")
}
