export function toDateString(d: Date): string {
  return `${d.getFullYear()}-${padLeft(''+(d.getMonth()+1), 2, '0')}-${padLeft(''+d.getDate(), 2, '0')}`
}

export function padLeft(str: string, length: number, pad = ' '): string {
    while (str.length < length) {
        str = pad + str;
    }
    return str;
}

export function padRight(str: string, length: number, pad = ' '): string {
    while (str.length < length) {
        str += pad;
    }
    return str;
}
