INSERT INTO crom_sites VALUES ('http://scp-wiki-cn.wikidot.com', 'SCP Wiki - Chinese', 'zh-CN', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://scp-wiki.wikidot.com', 'SCP Wiki - English', 'en', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://scp-jp.wikidot.com', 'SCP Wiki - Japanese', 'ja', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://backrooms-wiki-cn.wikidot.com', 'The Backrooms - Chinese', 'zh-CN', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://scpko.wikidot.com', 'SCP Wiki - Korean', 'ko', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://backrooms-wiki.wikidot.com', 'The Backrooms - English', 'en', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://scp-int.wikidot.com', 'SCP Wiki - International', 'en', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.314376+00');
INSERT INTO crom_sites VALUES ('http://fondationscp.wikidot.com', 'SCP Wiki - French', 'fr', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.542647+00');
INSERT INTO crom_sites VALUES ('http://scp-wiki-de.wikidot.com', 'SCP Wiki - German', 'de', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.542647+00');
INSERT INTO crom_sites VALUES ('http://lafundacionscp.wikidot.com', 'SCP Wiki - Spanish', 'es', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.542647+00');
INSERT INTO crom_sites VALUES ('http://scp-pl.wikidot.com', 'SCP Wiki - Polish', 'pl', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.542647+00');
INSERT INTO crom_sites VALUES ('http://scp-vn.wikidot.com', 'SCP Wiki - Vietnamese', 'vi', 0, NULL, NULL, NULL, NULL, NULL, NULL, '2026-11-03 15:14:50.542647+00');

INSERT INTO stock_symbols VALUES ('CLOUD', '云国指数', '云分', 'local', NULL, true, NULL, 0, '{"basePrice": 20.0}', '2026-11-03 15:14:49.932058+00');
INSERT INTO stock_symbols VALUES ('CN', '中分指数', '中分', 'crom', 'http://scp-wiki-cn.wikidot.com', false, NULL, 10, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('EN', '英分指数', '英分', 'crom', 'http://scp-wiki.wikidot.com', false, NULL, 20, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('JP', '日分指数', '日分', 'crom', 'http://scp-jp.wikidot.com', false, NULL, 30, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('BRCN', '中文后室指数', '中后室', 'crom', 'http://backrooms-wiki-cn.wikidot.com', false, NULL, 40, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('KO', '韩分指数', '韩分', 'crom', 'http://scpko.wikidot.com', false, NULL, 50, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('BREN', '英文后室指数', '英后室', 'crom', 'http://backrooms-wiki.wikidot.com', false, NULL, 70, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('INT', '国际站指数', '国际站', 'crom', 'http://scp-int.wikidot.com', false, NULL, 80, '{"basePrice": 20.0}', '2026-11-03 15:14:50.082976+00');
INSERT INTO stock_symbols VALUES ('FR', '法分指数', '法分', 'crom', 'http://fondationscp.wikidot.com', false, NULL, 60, '{"basePrice": 20.0}', '2026-11-03 15:14:50.545003+00');
INSERT INTO stock_symbols VALUES ('DE', '德分指数', '德分', 'crom', 'http://scp-wiki-de.wikidot.com', false, NULL, 90, '{"basePrice": 20.0}', '2026-11-03 15:14:50.545003+00');
INSERT INTO stock_symbols VALUES ('ES', '西分指数', '西分', 'crom', 'http://lafundacionscp.wikidot.com', false, NULL, 100, '{"basePrice": 20.0}', '2026-11-03 15:14:50.545003+00');
INSERT INTO stock_symbols VALUES ('PL', '波分指数', '波分', 'crom', 'http://scp-pl.wikidot.com', false, NULL, 110, '{"basePrice": 20.0}', '2026-11-03 15:14:50.545003+00');
INSERT INTO stock_symbols VALUES ('VN', '越南分指数', '越分', 'crom', 'http://scp-vn.wikidot.com', false, NULL, 120, '{"basePrice": 20.0}', '2026-11-03 15:14:50.545003+00');

INSERT INTO stock_bots VALUES ('random', 300, 6000, 0.0000, 0, '2026-11-03 15:14:49.249921+00', 'CLOUD');
INSERT INTO stock_bots VALUES ('contrarian', 300, 6000, 0.0000, 0, '2026-11-03 15:14:49.249921+00', 'CLOUD');
INSERT INTO stock_bots VALUES ('momentum', 300, 6000, 0.0000, 0, '2026-11-03 15:14:49.249921+00', 'CLOUD');

INSERT INTO stock_maker VALUES (999100, 0, 100.0000, 100.0000, 0, '2026-11-03 15:14:49.252739+00', 'CLOUD');

INSERT INTO sync_state VALUES ('gacha.params', '{"rates": {"n": 5250, "r": 3000, "sr": 1200, "ur": 150, "ssr": 400}, "costTen": 90, "pityCount": 60, "costSingle": 10, "tradeFeePct": 5, "tenGuarantee": "sr"}', '2026-11-03 15:14:47.900384+00');
INSERT INTO sync_state VALUES ('stock.maker', '{"bots": {"random": {"size": 40, "count": 2, "chancePct": 25}, "enabled": false, "momentum": {"size": 60, "count": 3, "thresholdPct": 1.2}, "contrarian": {"size": 80, "count": 3, "deviationPct": 8.0}}, "levels": 5, "enabled": true, "stepPct": 0.35, "sizePerLevel": 400}', '2026-11-03 15:14:48.82521+00');
INSERT INTO sync_state VALUES ('stock.params', '{"feePct": 0.5, "impact": 0.35, "minPrice": 0.1, "spreadPct": 0.8, "targetInv": 950000, "sigmaDaily": 0.02, "floatShares": 1000000, "dailyBandPct": 10, "halfLifeDays": 3.0, "inventoryEta": 0.006, "maxPositionPct": 2}', '2026-11-03 15:14:48.336641+00');

INSERT INTO task_defs VALUES ('daily_vote', 'daily', '评分一篇作品', '对任意一篇作品投出评分', 5, 1, 0, true);
INSERT INTO task_defs VALUES ('daily_post', 'daily', '发布一条帖子', '在论坛发布一条帖子', 5, 1, 1, true);
INSERT INTO task_defs VALUES ('daily_page', 'daily', '发布一个页面', '在站内创建一个新页面', 10, 1, 2, true);
INSERT INTO task_defs VALUES ('monthly_uv30', 'monthly', '「uv至上」作品达到 +30', '本月发布的一篇带「uv至上」标签的作品评分达到 +30', 20, 1, 0, true);
INSERT INTO task_defs VALUES ('monthly_dv30', 'monthly', '「dv至上」作品达到 -30', '本月发布的一篇带「dv至上」标签的作品评分达到 -30', 20, 1, 1, true);
INSERT INTO task_defs VALUES ('monthly_comments20', 'monthly', '本月作品收获 20 条他人评论', '本月发布的作品累计收到 20 条来自他人的评论', 20, 20, 2, true);
