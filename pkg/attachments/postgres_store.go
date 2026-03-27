package attachments

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements Store backed by PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Create(attachment Attachment) error {
	if attachment.ID == "" {
		return fmt.Errorf("attachment id is required")
	}
	if attachment.MissionID == "" {
		return fmt.Errorf("mission id is required")
	}
	if attachment.ThreadID == "" {
		return fmt.Errorf("thread id is required")
	}
	if attachment.Filename == "" {
		return fmt.Errorf("filename is required")
	}
	if attachment.AbsolutePath == "" {
		return fmt.Errorf("absolute path is required")
	}
	if attachment.FileCategory == "" {
		return fmt.Errorf("file category is required")
	}
	if attachment.Status == "" {
		attachment.Status = StatusActive
	}

	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO mission_attachments (
			attachment_id, mission_id, thread_id, uploaded_by_message_id,
			filename, content_type, size_bytes, relative_path, absolute_path,
			file_category, token_estimate, extracted_text, parent_attachment_id,
			status, created_at
		) VALUES (
			$1, $2, $3, NULLIF($4, ''),
			$5, $6, $7, $8, $9,
			$10, $11, $12, NULLIF($13, ''),
			$14, $15
		)
		ON CONFLICT (attachment_id) DO NOTHING
	`,
		attachment.ID, attachment.MissionID, attachment.ThreadID, attachment.UploadedByMessageID,
		attachment.Filename, attachment.ContentType, attachment.SizeBytes, attachment.RelativePath, attachment.AbsolutePath,
		string(attachment.FileCategory), attachment.TokenEstimate, attachment.ExtractedText, attachment.ParentAttachmentID,
		string(attachment.Status), attachment.CreatedAt,
	)
	return err
}

func (s *PostgresStore) Get(attachmentID string) (Attachment, error) {
	row := s.pool.QueryRow(context.Background(), `
		SELECT attachment_id, mission_id, thread_id, COALESCE(uploaded_by_message_id, ''),
			filename, content_type, size_bytes, relative_path, absolute_path,
			file_category, token_estimate, extracted_text, COALESCE(parent_attachment_id, ''),
			status, created_at
		FROM mission_attachments WHERE attachment_id = $1
	`, attachmentID)
	return scanAttachment(row)
}

func (s *PostgresStore) ListByMission(missionID string) ([]Attachment, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT attachment_id, mission_id, thread_id, COALESCE(uploaded_by_message_id, ''),
			filename, content_type, size_bytes, relative_path, absolute_path,
			file_category, token_estimate, extracted_text, COALESCE(parent_attachment_id, ''),
			status, created_at
		FROM mission_attachments
		WHERE mission_id = $1 AND status = 'active'
		ORDER BY created_at
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAttachments(rows)
}

func (s *PostgresStore) ListByThread(threadID string) ([]Attachment, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT attachment_id, mission_id, thread_id, COALESCE(uploaded_by_message_id, ''),
			filename, content_type, size_bytes, relative_path, absolute_path,
			file_category, token_estimate, extracted_text, COALESCE(parent_attachment_id, ''),
			status, created_at
		FROM mission_attachments
		WHERE thread_id = $1 AND status = 'active'
		ORDER BY created_at
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAttachments(rows)
}

// ListInheritedAttachments uses a recursive CTE to walk the mission parent chain
// and returns all active attachments from root to the target mission.
func (s *PostgresStore) ListInheritedAttachments(missionID string) ([]Attachment, error) {
	rows, err := s.pool.Query(context.Background(), `
		WITH RECURSIVE mission_chain AS (
			SELECT mission_id, parent_mission_id, 0 AS depth
			FROM missions WHERE mission_id = $1
			UNION ALL
			SELECT m.mission_id, m.parent_mission_id, mc.depth + 1
			FROM missions m
			JOIN mission_chain mc ON m.mission_id = mc.parent_mission_id
		)
		SELECT a.attachment_id, a.mission_id, a.thread_id, COALESCE(a.uploaded_by_message_id, ''),
			a.filename, a.content_type, a.size_bytes, a.relative_path, a.absolute_path,
			a.file_category, a.token_estimate, a.extracted_text, COALESCE(a.parent_attachment_id, ''),
			a.status, a.created_at
		FROM mission_attachments a
		JOIN mission_chain mc ON a.mission_id = mc.mission_id
		WHERE a.status = 'active'
		ORDER BY mc.depth DESC, a.created_at
	`, missionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectAttachments(rows)
}

func scanAttachment(row pgx.Row) (Attachment, error) {
	var a Attachment
	var category, status string
	err := row.Scan(
		&a.ID, &a.MissionID, &a.ThreadID, &a.UploadedByMessageID,
		&a.Filename, &a.ContentType, &a.SizeBytes, &a.RelativePath, &a.AbsolutePath,
		&category, &a.TokenEstimate, &a.ExtractedText, &a.ParentAttachmentID,
		&status, &a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Attachment{}, ErrAttachmentNotFound
		}
		return Attachment{}, err
	}
	a.FileCategory = FileCategory(category)
	a.Status = AttachmentStatus(status)
	return a, nil
}

func collectAttachments(rows pgx.Rows) ([]Attachment, error) {
	var result []Attachment
	for rows.Next() {
		var a Attachment
		var category, status string
		if err := rows.Scan(
			&a.ID, &a.MissionID, &a.ThreadID, &a.UploadedByMessageID,
			&a.Filename, &a.ContentType, &a.SizeBytes, &a.RelativePath, &a.AbsolutePath,
			&category, &a.TokenEstimate, &a.ExtractedText, &a.ParentAttachmentID,
			&status, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		a.FileCategory = FileCategory(category)
		a.Status = AttachmentStatus(status)
		result = append(result, a)
	}
	return result, rows.Err()
}
