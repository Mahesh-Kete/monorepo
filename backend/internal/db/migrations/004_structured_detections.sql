-- Store detection presentation fields separately from the raw message.
-- details/source are JSON blobs so the dashboard can render readable rows and
-- source code blocks without parsing a sentence.

ALTER TABLE detections ADD COLUMN title TEXT;
ALTER TABLE detections ADD COLUMN summary TEXT;
ALTER TABLE detections ADD COLUMN details TEXT;
ALTER TABLE detections ADD COLUMN source TEXT;
