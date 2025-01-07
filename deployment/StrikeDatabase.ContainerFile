FROM postgres:14.15

EXPOSE 5432

COPY config/init.sql /docker-entrypoint-initdb.d/
