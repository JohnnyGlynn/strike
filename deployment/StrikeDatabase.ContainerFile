FROM postgres:14.15

# TODO: Secrets from config
ENV POSTGRES_PASSWORD plaintextisbad
ENV POSTGRES_USER strikeadmin

ENV POSTGRES_DB strike

EXPOSE 5432

COPY config/init.sql /docker-entrypoint-initdb.d/
