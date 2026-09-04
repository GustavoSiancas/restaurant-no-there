ALTER TABLE worker_information
    ADD COLUMN photo_url TEXT;

ALTER TABLE worker_information
    ADD CONSTRAINT worker_information_photo_url_not_blank
    CHECK (photo_url IS NULL OR BTRIM(photo_url) <> '');
