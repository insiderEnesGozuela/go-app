-- down migration up'ın tam tersini yapar. golang-migrate `migrate down 1` ile
-- bunu çalıştırır. Index tabloyla birlikte düşeceği için ayrıca DROP INDEX'e
-- gerek yok, ama açıkça okunur olsun diye tabloyu drop ediyoruz.
DROP TABLE IF EXISTS users;
