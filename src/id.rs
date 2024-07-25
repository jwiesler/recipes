use unicode_normalization::UnicodeNormalization;

fn is_not_ok(r: char) -> bool {
    !(32..127).contains(&(r as u32))
        || !(r.is_alphabetic() || r.is_numeric() || r.is_whitespace() || r == '-')
}

enum Lower {
    One(char),
    Two(char, char),
}

fn map_german_umlauts(r: char) -> Lower {
    let lower = r.to_lowercase().next().unwrap();
    match lower {
        'ä' => Lower::Two('a', 'e'),
        'ö' => Lower::Two('o', 'e'),
        'ü' => Lower::Two('u', 'e'),
        'ß' => Lower::Two('s', 's'),
        _ => Lower::One(lower),
    }
}

fn replace_german_umlauts(input: &str) -> String {
    let mut result = String::with_capacity(input.len());
    for c in input.chars() {
        match map_german_umlauts(c) {
            Lower::One(c) => result.push(c),
            Lower::Two(a, b) => {
                result.push(a);
                result.push(b);
            }
        }
    }
    result
}

fn remove_matches(input: &str, mut f: impl FnMut(char) -> bool) -> String {
    let mut result = String::with_capacity(input.len());
    for c in input.chars() {
        if !f(c) {
            result.push(c);
        }
    }
    result
}

fn replace_space_and_collapse(input: &str, replacement: char) -> String {
    let mut result = String::with_capacity(input.len());
    let mut last_space = false;
    for c in input.chars() {
        if c.is_whitespace() || c == replacement {
            if !last_space {
                result.push(replacement);
            }
            last_space = true;
        } else {
            result.push(c);
            last_space = false;
        }
    }
    result
}

pub fn to_id_string(s: &str) -> String {
    let s = replace_german_umlauts(&s);
    let s: String = s.chars().nfkd().collect();
    let s = remove_matches(&s, is_not_ok);
    replace_space_and_collapse(&s, '-')
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test() {
        assert_eq!(to_id_string("Crêpe"), "crepe");
        assert_eq!(to_id_string("Grünkern"), "gruenkern");
        assert_eq!(to_id_string("Nasi Goreng"), "nasi-goreng");
    }
}
