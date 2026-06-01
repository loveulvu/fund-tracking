export default async function handler(req, res) {
  if (req.method !== 'POST') {
    res.status(405).json({ error: 'Method not allowed' });
    return;
  }

  const backendBaseUrl = process.env.GO_API_BASE_URL || process.env.NEXT_PUBLIC_GO_API_URL;
  const updateKey = process.env.UPDATE_API_KEY;

  if (!backendBaseUrl || !updateKey) {
    res.status(500).json({ error: 'Update proxy is not configured' });
    return;
  }

  const upstreamUrl = `${backendBaseUrl.replace(/\/$/, '')}/api/update/async`;
  let response;

  try {
    response = await fetch(upstreamUrl, {
      method: 'POST',
      headers: {
        'X-Update-Key': updateKey,
      },
    });
  } catch (error) {
    console.error('Update proxy upstream fetch failed', {
      upstreamUrl,
      error: describeProxyError(error),
    });
    res.status(502).json({ error: 'Update proxy upstream request failed' });
    return;
  }

  const text = await response.text();

  res.status(response.status);
  res.setHeader('Content-Type', response.headers.get('content-type') || 'application/json');
  res.send(text);
}

function describeProxyError(error) {
  if (!error || typeof error !== 'object') {
    return {
      name: undefined,
      message: String(error),
      code: undefined,
      cause: undefined,
    };
  }

  return {
    name: error.name,
    message: error.message,
    code: error.code,
    cause: error.cause,
  };
}
