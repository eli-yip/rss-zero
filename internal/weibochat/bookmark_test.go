package weibochat

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBookmarkBlocksOwnershipAtomicityAndChronology(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	earlier := now.Add(-time.Hour)
	messages := []Message{
		{GroupID: "1", MessageID: "20", SentAt: &now, Raw: []byte(`{}`)},
		{GroupID: "1", MessageID: "9", SentAt: &now, Raw: []byte(`{}`)},
		{GroupID: "2", MessageID: "9", SentAt: &now, Raw: []byte(`{}`)},
		{GroupID: "1", MessageID: "100", SentAt: &earlier, Raw: []byte(`{}`)},
		{GroupID: "1", MessageID: "1", Raw: []byte(`{}`)},
		{GroupID: "1", MessageID: "2", Raw: []byte(`{}`)},
	}
	require.NoError(t, db.Create(&messages).Error)
	refs := []MessageRef{}
	for _, m := range messages {
		refs = append(refs, MessageRef{m.GroupID, string(m.MessageID)})
	}
	block, err := s.CreateBookmarkBlock(ctx, "alice", "100%_技术", []string{"技术", "技术", "讨论"}, refs)
	require.NoError(t, err)
	require.EqualValues(t, 6, block.MessageCount)
	require.Len(t, block.Tags, 2)
	_, err = s.GetBookmarkBlock(ctx, "bob", block.ID)
	require.ErrorIs(t, err, ErrBookmarkNotFound)
	require.ErrorIs(t, s.AddBookmarkMessages(ctx, "bob", block.ID, refs), ErrBookmarkNotFound)
	require.ErrorIs(t, s.RemoveBookmarkMessage(ctx, "bob", block.ID, refs[0]), ErrBookmarkNotFound)
	require.ErrorIs(t, s.DeleteBookmarkBlock(ctx, "bob", block.ID), ErrBookmarkNotFound)
	title := "changed"
	_, err = s.UpdateBookmarkBlock(ctx, "bob", block.ID, &title, nil)
	require.ErrorIs(t, err, ErrBookmarkNotFound)
	_, err = s.ListBookmarkMessages(ctx, "bob", block.ID, "", 1)
	require.ErrorIs(t, err, ErrBookmarkNotFound)
	require.NoError(t, s.AddBookmarkMessages(ctx, "alice", block.ID, refs))
	same, err := s.GetBookmarkBlock(ctx, "alice", block.ID)
	require.NoError(t, err)
	require.Equal(t, block.UpdatedAt, same.UpdatedAt)
	var got []MessageRef
	cursor := ""
	for {
		page, e := s.ListBookmarkMessages(ctx, "alice", block.ID, cursor, 1)
		require.NoError(t, e)
		for _, m := range page.Messages {
			got = append(got, MessageRef{m.GroupID, string(m.MessageID)})
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	require.Equal(t, []MessageRef{{"1", "100"}, {"1", "9"}, {"2", "9"}, {"1", "20"}, {"1", "1"}, {"1", "2"}}, got)
	_, err = s.ListBookmarkMessages(ctx, "alice", block.ID, "invalid", 10)
	require.ErrorIs(t, err, ErrInvalidBookmarkCursor)
	_, err = s.CreateBookmarkBlock(ctx, "alice", "bad", nil, []MessageRef{{"1", "999"}})
	require.ErrorIs(t, err, ErrMessageNotFound)
	var n int64
	require.NoError(t, db.Model(&BookmarkBlock{}).Count(&n).Error)
	require.EqualValues(t, 1, n)
	counts, err := s.GetBookmarkCounts(ctx, "alice", "1", []string{"9"})
	require.NoError(t, err)
	require.EqualValues(t, 1, counts["9"])
	counts, err = s.GetBookmarkCounts(ctx, "bob", "1", []string{"9"})
	require.NoError(t, err)
	require.Empty(t, counts)
	page, err := s.ListBookmarkBlocks(ctx, "alice", BookmarkBlockFilter{Title: "%_", Tags: []string{"技术", "讨论"}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, page.Blocks, 1)
	page, err = s.ListBookmarkBlocks(ctx, "bob", BookmarkBlockFilter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, page.Blocks)
	for _, ref := range refs {
		require.NoError(t, s.RemoveBookmarkMessage(ctx, "alice", block.ID, ref))
	}
	empty, err := s.GetBookmarkBlock(ctx, "alice", block.ID)
	require.NoError(t, err)
	require.Zero(t, empty.MessageCount)
	require.Equal(t, block.Tags, empty.Tags)
	require.NoError(t, s.DeleteBookmarkBlock(ctx, "alice", block.ID))
	require.NoError(t, db.Model(&Message{}).Count(&n).Error)
	require.EqualValues(t, 6, n)
	tags, err := s.ListBookmarkTags(ctx, "alice")
	require.NoError(t, err)
	require.Empty(t, tags)
}

func TestBookmarkBlockPaginationAndAddition(t *testing.T) {
	s, db := testStore(t)
	ctx := context.Background()
	require.NoError(t, db.Create(&Message{GroupID: "1", MessageID: "1", Raw: []byte(`{}`)}).Error)
	ref := MessageRef{"1", "1"}
	a, err := s.CreateBookmarkBlock(ctx, "a", "a", nil, []MessageRef{ref})
	require.NoError(t, err)
	b, err := s.CreateBookmarkBlock(ctx, "a", "b", nil, []MessageRef{ref})
	require.NoError(t, err)
	// Equal timestamps exercise UUID tie breaking.
	require.NoError(t, db.Model(&BookmarkBlock{}).Where("id = ?", b.ID).Update("updated_at", a.UpdatedAt).Error)
	first, err := s.ListBookmarkBlocks(ctx, "a", BookmarkBlockFilter{Limit: 1, Message: &ref})
	require.NoError(t, err)
	require.Len(t, first.Blocks, 1)
	require.NotEmpty(t, first.NextCursor)
	second, err := s.ListBookmarkBlocks(ctx, "a", BookmarkBlockFilter{Limit: 1, Cursor: first.NextCursor, Message: &ref})
	require.NoError(t, err)
	require.Len(t, second.Blocks, 1)
	require.NotEqual(t, first.Blocks[0].ID, second.Blocks[0].ID)
	require.Empty(t, second.NextCursor)
	require.NoError(t, s.RemoveBookmarkMessage(ctx, "a", a.ID, ref))
	before, err := s.GetBookmarkBlock(ctx, "a", a.ID)
	require.NoError(t, err)
	require.ErrorIs(t, s.AddBookmarkMessages(ctx, "a", a.ID, []MessageRef{ref, {"1", "2"}}), ErrMessageNotFound)
	after, err := s.GetBookmarkBlock(ctx, "a", a.ID)
	require.NoError(t, err)
	require.Zero(t, after.MessageCount)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt)
	require.NoError(t, s.AddBookmarkMessages(ctx, "a", a.ID, []MessageRef{ref, ref}))
	after, err = s.GetBookmarkBlock(ctx, "a", a.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, after.MessageCount)
}
