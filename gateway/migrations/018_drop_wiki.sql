-- Migration 018: drop the wiki's tables.
--
-- The wiki was deleted on 2026-09-27 (Greg: "remove wiki features", "delete
-- them too", and on the data: "no wiki users"). Nothing reads or writes these
-- tables any more, and a fresh database no longer creates them, so a database
-- from before today is the only place they can still exist. Dropping them
-- there makes every install the same shape.
--
-- IF EXISTS on every statement: a database created after the wiki left never
-- had them, and this must be a no-op there.
DROP TABLE IF EXISTS wiki_page_refs;
DROP TABLE IF EXISTS wiki_claims;
DROP TABLE IF EXISTS wiki_findings;
DROP TABLE IF EXISTS wiki_pages;
DROP TABLE IF EXISTS wikis;
-- Two more from older versions still sit in long-lived databases; nothing in
-- the current code creates them.
DROP TABLE IF EXISTS wiki_members;
DROP TABLE IF EXISTS wiki_settings;
