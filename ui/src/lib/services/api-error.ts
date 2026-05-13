export async function throwIfNotOk(response: Response, defaultMessage: string): Promise<void> {
	if (response.ok) return;

	let body = '';
	try {
		body = (await response.text()).trim();
	} catch {
		// ignore body read errors
	}

	const suffix = body ? `: ${body}` : '';
	throw new Error(`${defaultMessage} (HTTP ${response.status})${suffix}`);
}
