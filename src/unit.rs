const SI_UNITS: [char; 3] = ['g', 'l', 'm'];
const SI_UNIT_PREFIXES: [char; 5] = ['k', 'd', 'c', 'm', 'µ'];

pub(crate) fn unit_needs_space(s: &str) -> bool {
    let mut chars = s.chars();
    let Some(ch) = chars.next() else {
        return false;
    };

    let ch = if SI_UNIT_PREFIXES.contains(&ch) {
        match chars.next() {
            None => return true,
            Some(ch) => ch,
        }
    } else {
        ch
    };
    !SI_UNITS.contains(&ch)
}

#[test]
fn test() {
    assert!(unit_needs_space("a"));
    assert!(unit_needs_space("k"));
    assert!(!unit_needs_space("kg"));
    assert!(!unit_needs_space("g"));
}
