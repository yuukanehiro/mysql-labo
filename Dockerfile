FROM mysql:8.0

ENV MYSQL_ROOT_PASSWORD=root
ENV MYSQL_DATABASE=test_db
ENV MYSQL_USER=user
ENV MYSQL_PASSWORD=pass

# 初期化SQLファイルのコピー（任意）
COPY ./init.sql /docker-entrypoint-initdb.d/
