# M3U8 Cross-Origin Proxy Server (Go)

A high-performance proxy server written in Go for handling M3U8 playlists, TS segments, MP4 videos, and general content with CORS support.

## Features

- **M3U8 Playlist Proxying**: Rewrites M3U8 playlists to proxy all segments and encryption keys
- **TS Segment Proxying**: Handles video segments with proper content types
- **MP4 Video Proxying**: Supports range requests for video streaming
- **Generic Fetch**: Proxy any content with optional referer header
- **CORS Support**: Configurable allowed origins
- **Domain-Specific Headers**: Automatically generates appropriate headers based on target domain

## Installation

1. Clone or download this repository
2. Install dependencies:
```bash
go mod download
```

## Configuration

Create or edit the `.env` file:

```env
PUBLIC_URL=http://localhost:3000
HOST=localhost
PORT=3000
# ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3001
```

- `PUBLIC_URL`: The public URL of your proxy server
- `HOST`: Host to bind the server to
- `PORT`: Port to run the server on
- `ALLOWED_ORIGINS`: Comma-separated list of allowed origins (leave commented for all origins)

## Running the Server

### Development
```bash
go run .
```

### Build and Run
```bash
go build -o proxy-server
./proxy-server
```

### Using Docker
```bash
docker build -t go-proxy .
docker run -p 3000:3000 --env-file .env go-proxy
```

## API Endpoints

### 1. M3U8 Proxy
```
GET /proxy?url={m3u8_url}&headers={optional_headers}
```
Proxies M3U8 playlists and rewrites all URLs to go through the proxy.

### 2. TS Segment Proxy
```
GET /ts-proxy?url={ts_segment_url}&headers={optional_headers}
```
Proxies video segments, encryption keys, and other media files.

### 3. MP4 Proxy
```
GET /mp4-proxy?url={mp4_url}&headers={optional_headers}
```
Proxies MP4 videos with support for range requests (seeking).

### 4. Generic Fetch
```
GET /fetch?url={any_url}&ref={optional_referer}
```
Proxies any content with an optional referer header.

### 5. Videostr Proxy (Catch-all)
```
GET /{url_without_https}
```
Proxies any URL without the `https://` prefix. Automatically adds videostr.net specific headers:
- `Referer: https://videostr.net/`
- `Origin: https://videostr.net/`

**Example**: `http://localhost:3000/example.com/video.m3u8` will proxy `https://example.com/video.m3u8`

## Usage Examples

### Proxying an M3U8 Playlist
```javascript
const proxyUrl = 'http://localhost:3000/proxy';
const m3u8Url = 'https://example.com/playlist.m3u8';
const headers = { 'Authorization': 'Bearer token' };

const url = `${proxyUrl}?url=${encodeURIComponent(m3u8Url)}&headers=${encodeURIComponent(JSON.stringify(headers))}`;

// Use this URL in your video player
```

### Proxying an MP4 Video
```javascript
const proxyUrl = 'http://localhost:3000/mp4-proxy';
const mp4Url = 'https://example.com/video.mp4';

const url = `${proxyUrl}?url=${encodeURIComponent(mp4Url)}`;

// Use this URL in your video player
```

### Using the Fetch Endpoint
```javascript
const proxyUrl = 'http://localhost:3000/fetch';
const imageUrl = 'https://example.com/image.jpg';
const referer = 'https://example.com';

const url = `${proxyUrl}?url=${encodeURIComponent(imageUrl)}&ref=${encodeURIComponent(referer)}`;
```

### Using the Videostr Proxy (Catch-all)
```javascript
// For URLs that need videostr.net headers
const videoUrl = 'example.com/path/to/video.m3u8';
const proxyUrl = `http://localhost:3000/${videoUrl}`;

// This will automatically add:
// - Referer: https://videostr.net/
// - Origin: https://videostr.net/
// And proxy: https://example.com/path/to/video.m3u8
```

## How It Works

1. **Request Validation**: Validates the target URL and optional headers
2. **Header Generation**: Generates appropriate headers based on the target domain
3. **Content Fetching**: Fetches content from the target URL
4. **Content Rewriting** (for M3U8): Rewrites all URLs in playlists to go through the proxy
5. **Response Streaming**: Streams the content back to the client with proper headers

## Security Considerations

- Configure `ALLOWED_ORIGINS` to restrict which domains can use your proxy
- Consider adding authentication if deploying publicly
- Monitor usage to prevent abuse
- Use HTTPS in production

## License

MIT
