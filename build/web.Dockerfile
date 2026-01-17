FROM nginxinc/nginx-unprivileged:1.27-alpine
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY web/index.html web/style.css web/app.js /usr/share/nginx/html/
EXPOSE 8080
