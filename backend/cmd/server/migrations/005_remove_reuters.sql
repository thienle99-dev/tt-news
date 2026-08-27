-- Remove the retired Reuters source and its previously imported articles.
DELETE FROM sources WHERE feed_url = 'https://www.reuters.com/sitemap/';
