CREATE OR REPLACE VIEW crom_content_pages AS
 SELECT url,
    site_url,
    category,
    created_at,
    rev_count,
    vote_count,
    rating,
    comments,
    is_private,
    fp,
    detail_fp,
    detail_at,
    seen_at,
    title,
    alt_title
   FROM crom_pages
  WHERE ((NOT is_private) AND (category <> ALL (ARRAY['fragment'::text, 'component'::text, 'theme'::text, 'admin'::text, 'system'::text, 'nav'::text, 'forum'::text, 'workbench'::text, 'sandbox'::text, 'draft'::text, '_template'::text, 'more-by'::text, 'review'::text, 'criticism'::text])));
