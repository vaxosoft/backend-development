# syntax=docker/dockerfile:1
# Build context must be languages/csharp (it holds Directory.Build.props, Directory.Packages.props, global.json):
#   docker build -f modules/14-packaging-and-running/examples/Dockerfile --build-arg GIT_SHA=$(git rev-parse --short HEAD) -t task-service .
#   docker run --rm -p 8080:8080 task-service

FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src

# Restore first, from the project file only, so this layer stays cached until dependencies change.
COPY global.json Directory.Build.props Directory.Packages.props ./
COPY modules/14-packaging-and-running/examples/M14.Examples.csproj modules/14-packaging-and-running/examples/
RUN dotnet restore modules/14-packaging-and-running/examples/M14.Examples.csproj -r linux-x64

COPY modules/14-packaging-and-running/examples/ modules/14-packaging-and-running/examples/
ARG GIT_SHA=local
RUN dotnet publish modules/14-packaging-and-running/examples/M14.Examples.csproj \
    -c Release -r linux-x64 --no-restore --self-contained false \
    -p:UseAppHost=false -p:SourceRevisionId=${GIT_SHA} -o /out

# The runtime image has no SDK, shell tooling or build output other than /out.
FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS runtime
WORKDIR /app
COPY --from=build /out ./

ENV ADDR=0.0.0.0:8080
EXPOSE 8080

# The official image ships a non-root user named "app" (UID 1654).
USER app
ENTRYPOINT ["dotnet", "M14.Examples.dll", "04-task-service"]
