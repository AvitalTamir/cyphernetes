// This is a mock API for now. In a real application, this would make HTTP requests to your backend.

export interface QueryResponse {
  result: string;
  graph: string; // Change this to string as it should be a JSON string
}

export async function executeQuery(query: string): Promise<QueryResponse> {
  const response = await fetch('/api/query', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ query }),
  });

  if (!response.ok) {
    throw new Error('Failed to execute query');
  }

  const data = await response.json();
  console.log("API response:", data); // Add this line for debugging

  return data;
}