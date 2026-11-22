-- The size of typed text in CSS pixels at 100% zoom; it grows with the zoom
-- like the page. Text typed before sizes could be chosen keeps the size it
-- was shown at. A constant default makes this a catalog-only change.
ALTER TABLE text_boxes
    ADD COLUMN font_size integer NOT NULL DEFAULT 14,
    ADD CONSTRAINT text_boxes_font_size_range CHECK (font_size BETWEEN 8 AND 48);
