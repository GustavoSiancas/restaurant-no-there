ALTER TABLE worker_information
    DROP CONSTRAINT IF EXISTS worker_information_photo_url_not_blank,
    DROP COLUMN IF EXISTS photo_url;
