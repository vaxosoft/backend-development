# syntax=docker/dockerfile:1

FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src
# Copy the project file first so the restore layer is cached until dependencies change.
COPY P7.TestedService.csproj ./
RUN dotnet restore
COPY . .
RUN dotnet publish P7.TestedService.csproj -c Release -o /app --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS final
WORKDIR /app
# The service reads PORT (and ENV, LOG_LEVEL); 8080 also works for a non-root user.
ENV PORT=8080 \
    ENV=prod \
    LOG_LEVEL=info
EXPOSE 8080
COPY --from=build /app .
# The official image ships an unprivileged user called "app".
USER app
# Probe: GET /healthz (liveness) and GET /readyz (readiness); the aspnet image has no curl, so the orchestrator does it.
ENTRYPOINT ["dotnet", "P7.TestedService.dll"]
